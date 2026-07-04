package http

import (
	"fmt"
	"sort"

	"github.com/InTacht/xqua-go/pkg/errors"

	"github.com/gofiber/fiber/v3"
)

// StatusMap maps catalog entries to HTTP status codes for a route or group.
// Lookups match by error identity (errors.Is), so wrapped catalog errors and
// clones resolve to the same status as their catalog entry.
type StatusMap map[*errors.Error]int

// ErrorFunc fully handles an error returned by a route handler. Returning a
// non-nil error defers to the transport's global error handler.
type ErrorFunc func(c fiber.Ctx, err error) error

// RouteOption configures per-route (or per-group) error handling. Options are
// composable: group options are inherited by child routes and groups, with
// route options applied on top.
type RouteOption func(*routeErr)

type routeErr struct {
	statuses StatusMap
	custom   ErrorFunc
}

// Status maps a single catalog entry to an HTTP status. Repeatable.
func Status(entry *errors.Error, code int) RouteOption {
	return func(re *routeErr) {
		if re.statuses == nil {
			re.statuses = StatusMap{}
		}
		re.statuses[entry] = code
	}
}

// Statuses maps several catalog entries to HTTP status codes at once.
func Statuses(m StatusMap) RouteOption {
	return func(re *routeErr) {
		if re.statuses == nil {
			re.statuses = StatusMap{}
		}
		for entry, code := range m {
			re.statuses[entry] = code
		}
	}
}

// OnError installs a full custom error handler for the route or group. It takes
// precedence over any Status/Statuses mapping.
func OnError(fn ErrorFunc) RouteOption {
	return func(re *routeErr) { re.custom = fn }
}

func buildRouteErr(opts []RouteOption) routeErr {
	var re routeErr
	for _, opt := range opts {
		if opt != nil {
			opt(&re)
		}
	}
	return re
}

// Router wraps a fiber.Router and applies per-route error resolution. Handlers
// return catalog errors; the router maps them to an HTTP status and renders the
// envelope. Errors that are not public-catalog entries (plain errors, internal
// catalog errors) bubble to the transport's global error handler.
//
// Status resolution for a returned public-catalog error is, in order:
//   - an explicit per-route/group Status/Statuses mapping (wins);
//   - the kind→status table (Config.KindStatuses / DefaultKindStatuses);
//   - Config.DefaultStatus (unknown kind).
//
// When several catalog errors are returned together, the highest resolved
// status is used. Status mappings may only reference entries of the transport's
// public catalog (Config.Catalog); registering a foreign entry panics.
type Router struct {
	router  fiber.Router
	catalog *errors.Catalog
	base    []RouteOption

	kindStatuses  KindStatuses
	defaultStatus int
	prefix        string
	rec           *recorder
}

func newRouter(r fiber.Router, catalog *errors.Catalog, kindStatuses KindStatuses, defaultStatus int, prefix string, rec *recorder) *Router {
	return &Router{
		router:        r,
		catalog:       catalog,
		kindStatuses:  kindStatuses,
		defaultStatus: defaultStatus,
		prefix:        prefix,
		rec:           rec,
	}
}

// statusFor resolves the HTTP status for a single public-catalog error:
// explicit mapping wins, then the kind table, then DefaultStatus.
func (r *Router) statusFor(e *errors.Error, statuses StatusMap) int {
	for entry, code := range statuses {
		if errors.Is(e, entry) {
			return code
		}
	}
	return kindStatus(r.kindStatuses, e.Kind, r.defaultStatus)
}

// resolveStatus returns the highest resolved status across entries.
func (r *Router) resolveStatus(entries errors.Errors, statuses StatusMap) int {
	best := 0
	for _, e := range entries {
		if s := r.statusFor(e, statuses); s > best {
			best = s
		}
	}
	if best == 0 {
		best = r.defaultStatus
	}
	return best
}

// handle applies route-level error handling to a non-nil handler error. Only
// errors whose every top-level entry belongs to the public catalog are rendered
// here; everything else bubbles so the global handler can replace/log it.
func (r *Router) handle(c fiber.Ctx, err error, re routeErr) error {
	if re.custom != nil {
		return re.custom(c, err)
	}
	entries := errors.AsErrors(err)
	if len(entries) == 0 {
		return err
	}
	for _, e := range entries {
		if !r.catalog.Contains(e) {
			return err
		}
	}
	return RES(c).Apply(err).Status(r.resolveStatus(entries, re.statuses)).Ok()
}

// Group creates a sub-router under prefix. Its options are inherited by every
// route and nested group registered on it.
func (r *Router) Group(prefix string, opts ...RouteOption) *Router {
	return &Router{
		router:        r.router.Group(prefix),
		catalog:       r.catalog,
		base:          mergeOptions(r.base, opts),
		kindStatuses:  r.kindStatuses,
		defaultStatus: r.defaultStatus,
		prefix:        joinPath(r.prefix, prefix),
		rec:           r.rec,
	}
}

// Handle registers handler for method and path with optional error handling.
// Panics when a Status/Statuses option references an entry that is not part of
// the transport's public catalog.
func (r *Router) Handle(method, path string, handler fiber.Handler, opts ...RouteOption) *Router {
	re := buildRouteErr(mergeOptions(r.base, opts))
	r.validateStatuses(method, path, re.statuses)
	r.record(method, path, re.statuses)
	r.router.Add([]string{method}, path, r.wrap(handler, re))
	return r
}

// record appends this route's declared error contract to the manifest.
func (r *Router) record(method, path string, statuses StatusMap) {
	specs := make([]ErrorSpec, 0, len(statuses))
	for entry := range statuses {
		specs = append(specs, errorSpecFrom(entry, r.statusFor(entry, statuses)))
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Code < specs[j].Code })
	r.rec.add(RouteSpec{Method: method, Path: joinPath(r.prefix, path), Errors: specs})
}

func (r *Router) validateStatuses(method, path string, statuses StatusMap) {
	for entry := range statuses {
		if entry == nil {
			panic(fmt.Sprintf("http: %s %s: nil catalog entry in status map", method, path))
		}
		if !r.catalog.Contains(entry) {
			panic(fmt.Sprintf(
				"http: %s %s: status map entry %s<%s> is not in the public catalog %q; map internal errors at the boundary",
				method, path, entry.Kind, entry.Code, r.catalog.Name(),
			))
		}
	}
}

// Get registers a GET route with optional error handling.
func (r *Router) Get(path string, handler fiber.Handler, opts ...RouteOption) *Router {
	return r.Handle(fiber.MethodGet, path, handler, opts...)
}

// Post registers a POST route with optional error handling.
func (r *Router) Post(path string, handler fiber.Handler, opts ...RouteOption) *Router {
	return r.Handle(fiber.MethodPost, path, handler, opts...)
}

// Put registers a PUT route with optional error handling.
func (r *Router) Put(path string, handler fiber.Handler, opts ...RouteOption) *Router {
	return r.Handle(fiber.MethodPut, path, handler, opts...)
}

// Patch registers a PATCH route with optional error handling.
func (r *Router) Patch(path string, handler fiber.Handler, opts ...RouteOption) *Router {
	return r.Handle(fiber.MethodPatch, path, handler, opts...)
}

// Delete registers a DELETE route with optional error handling.
func (r *Router) Delete(path string, handler fiber.Handler, opts ...RouteOption) *Router {
	return r.Handle(fiber.MethodDelete, path, handler, opts...)
}

// Fiber returns the underlying fiber.Router for advanced registration.
func (r *Router) Fiber() fiber.Router {
	return r.router
}

func (r *Router) wrap(handler fiber.Handler, re routeErr) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := handler(c); err != nil {
			return r.handle(c, err, re)
		}
		return nil
	}
}

func mergeOptions(base, opts []RouteOption) []RouteOption {
	if len(base) == 0 {
		return opts
	}
	merged := make([]RouteOption, 0, len(base)+len(opts))
	merged = append(merged, base...)
	merged = append(merged, opts...)
	return merged
}
