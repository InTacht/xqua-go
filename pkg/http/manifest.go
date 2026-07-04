package http

import (
	"sort"
	"strings"
	"sync"

	"github.com/InTacht/xqua-go/pkg/errors"
)

// EnvelopeVersion identifies the JSON response envelope contract. It is bumped
// when the wire shape of the envelope changes, and is embedded in the Manifest
// so downstream code generators can pin to a known envelope layout.
const EnvelopeVersion = "1"

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

// RouteSpec is the manifest description of a registered route and the public
// catalog errors it maps, each with its resolved HTTP status.
type RouteSpec struct {
	Method string      `json:"method"`
	Path   string      `json:"path"`
	Errors []ErrorSpec `json:"errors,omitempty"`
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
