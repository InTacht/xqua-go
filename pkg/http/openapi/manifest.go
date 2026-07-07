package openapi

import (
	"sort"
	"strings"
	"sync"

	"github.com/InTacht/xqua-go/pkg/errors"
)

// ErrorSpec is the manifest description of a single catalog error, including
// the HTTP status the transport resolves for it. It is the unit consumed by
// future OpenAPI/TypeScript generation.
type ErrorSpec struct {
	Kind       string `json:"kind"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Source     string `json:"source,omitempty"`
	HTTPStatus int    `json:"http_status"`
}

// RouteSpec is the manifest description of a registered route: its OpenAPI
// operation metadata, declared public catalog errors (each with its resolved
// HTTP status), and the named path parameters derived from the route pattern.
//
// Documented is true only when the route set a contract field on openapi.Route /
// GroupConfig (Summary, Description, OperationID, Request, Requests, Responses
// with explicit success or Err cases, or Extra). Tags and Specs alone do not
// document a bare handler. OpenAPI generation includes only documented routes —
// bare handlers and built-in ops (/health, /version) stay in the manifest for
// bookkeeping but are omitted from generated documents.
//
// Specs lists the OpenAPI document membership tags a route opts into; Manifest.
// ForSpec uses them alongside the path prefix to filter routes per document.
//
// The reflect-driven request/response declaration is carried in an unexported
// field consumed by document generation; it is not part of the serialized
// manifest.
type RouteSpec struct {
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	OperationID string      `json:"operation_id,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Specs       []string    `json:"specs,omitempty"`
	Params      []string    `json:"params,omitempty"`
	Errors      []ErrorSpec `json:"errors,omitempty"`
	Documented  bool        `json:"documented,omitempty"`

	op operationDecl
}

// Manifest is the transport's registration-time bookkeeping: every registered
// route, the full public catalog, and the envelope version. It is pure data,
// produced so future tooling can emit OpenAPI schemas or typed clients without
// re-deriving status resolution.
type Manifest struct {
	EnvelopeVersion string      `json:"envelope_version"`
	Routes          []RouteSpec `json:"routes"`
	Catalog         []ErrorSpec `json:"catalog"`
}

// ForSpec narrows the manifest to one OpenAPI document: only Documented routes
// whose path is under prefix (segment-aware) and whose Specs intersect tags are
// kept, and the catalog is subset to the errors those routes reference. An
// empty prefix or empty tags disables that filter.
func (m Manifest) ForSpec(prefix string, tags []string) Manifest {
	prefix = normalizePrefix(prefix)

	var routes []RouteSpec
	used := map[string]bool{}
	for _, r := range m.Routes {
		if !r.Documented || !matchesPrefix(r.Path, prefix) || !matchesTags(r.Specs, tags) {
			continue
		}
		routes = append(routes, r)
		for _, e := range r.Errors {
			used[e.Code] = true
		}
	}

	var catalog []ErrorSpec
	for _, e := range m.Catalog {
		if used[e.Code] {
			catalog = append(catalog, e)
		}
	}

	return Manifest{
		EnvelopeVersion: m.EnvelopeVersion,
		Routes:          routes,
		Catalog:         catalog,
	}
}

// normalizePrefix trims a path prefix to a clean, comparable form ("" and "/"
// both mean "no prefix filter").
func normalizePrefix(prefix string) string {
	if prefix == "" || prefix == "/" {
		return ""
	}
	return "/" + strings.Trim(prefix, "/")
}

// matchesPrefix reports whether path lies under prefix on segment boundaries:
// "/mobile" matches "/mobile" and "/mobile/v1" but not "/mobilepay". An empty
// prefix matches everything.
func matchesPrefix(path, prefix string) bool {
	if prefix == "" {
		return true
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// matchesTags reports whether the route's membership tags intersect want. Empty
// want disables the tag filter.
func matchesTags(routeSpecs, want []string) bool {
	if len(want) == 0 {
		return true
	}
	for _, w := range want {
		for _, rs := range routeSpecs {
			if rs == w {
				return true
			}
		}
	}
	return false
}

// recorder accumulates RouteSpecs as routes register. It is shared (by pointer)
// across a transport's routers and groups so the whole route tree lands in one
// manifest.
type recorder struct {
	mu     sync.Mutex
	routes []RouteSpec
}

func (r *recorder) add(spec RouteSpec) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.routes = append(r.routes, spec)
	r.mu.Unlock()
}

func (r *recorder) snapshot() []RouteSpec {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]RouteSpec, len(r.routes))
	copy(out, r.routes)
	return out
}

// errorSpecFrom builds an ErrorSpec from a catalog entry and its resolved status.
func errorSpecFrom(e *errors.Error, status int) ErrorSpec {
	return ErrorSpec{
		Kind:       e.Kind,
		Code:       e.Code,
		Message:    e.Message,
		Source:     e.Source,
		HTTPStatus: status,
	}
}

// joinPath joins a group prefix and a route path into a single clean path.
func joinPath(prefix, path string) string {
	switch {
	case prefix == "" || prefix == "/":
		prefix = ""
	default:
		prefix = "/" + strings.Trim(prefix, "/")
	}
	path = strings.Trim(path, "/")
	if path == "" {
		if prefix == "" {
			return "/"
		}
		return prefix
	}
	return prefix + "/" + path
}

// pathParams extracts named path parameters (Fiber ":name" segments, optional
// ":name?" included) from a route path, in order of appearance. Wildcard
// segments ("*", "+") have no stable name and are not recorded.
func pathParams(path string) []string {
	var params []string
	for seg := range strings.SplitSeq(path, "/") {
		if strings.HasPrefix(seg, ":") {
			params = append(params, strings.TrimSuffix(seg[1:], "?"))
		}
	}
	return params
}

// catalogSpecs renders every entry of catalog as an ErrorSpec, resolving each
// entry's status from the kind table (or defaultStatus) — the same logic the
// Router applies when no explicit route mapping exists.
func catalogSpecs(catalog *errors.Catalog, kindStatuses KindStatuses, defaultStatus int) []ErrorSpec {
	entries := catalog.Entries()
	specs := make([]ErrorSpec, 0, len(entries))
	for _, e := range entries {
		specs = append(specs, errorSpecFrom(e, kindStatus(kindStatuses, e.Kind, defaultStatus)))
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Code < specs[j].Code })
	return specs
}

// kindStatus resolves a status for a kind, falling back to defaultStatus.
func kindStatus(kindStatuses KindStatuses, kind string, defaultStatus int) int {
	if code, ok := kindStatuses[kind]; ok {
		return code
	}
	return defaultStatus
}
