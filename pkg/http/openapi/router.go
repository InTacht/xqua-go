package openapi

import (
	"reflect"
	"sort"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/compile"

	"github.com/gofiber/fiber/v3"
)

// Route declares one OpenAPI operation. Path is set by Route(path) or
// Describe(path); do not set it on this struct.
type Route struct {
	// Handler is func(context.Context, In) (Out, error). Required for live routes.
	Handler any

	// Request is the input shape for Describe routes (no Handler).
	Request any

	// Responses declares success (Returns / Returns(T)) and errors (Err).
	Responses *ResponseSet

	// Extra declares non-JSON success responses (101, streams) for OpenAPI only.
	Extra []ResponseDecl

	// Requests overrides request body declaration (multipart, etc.).
	Requests []ContentUnit

	Summary      string
	Description  string
	OperationID  string
	Tags         []string
	Specs        []string
	Deprecated   bool
	Hidden       bool
	ExternalDocs *ExternalDocs
	Security     SecuritySpec
}

// ResponseDecl declares one OpenAPI response for Extra or webhooks.
type ResponseDecl struct {
	Status      int
	Default     bool
	Body        any
	ContentType string
	Description string
	Summary     string
	ItemBody    any
	Encoding    map[string]string
	Example     any
	Examples    map[string]*Example
	Required    bool
}

// GroupConfig declares a path prefix and inherited contract defaults.
type GroupConfig struct {
	Prefix      string
	Responses   *ResponseSet
	Summary     string
	Description string
	Tags        []string
	Specs       []string
	Deprecated  bool
	Hidden      bool
	Security    SecuritySpec
}

type routeContract struct {
	responses       *ResponseSet
	explicitRequest any
	summary         string
	description     string
	operationID     string
	tags            []string
	specs           []string
	deprecated      bool
	hidden          bool
	externalDocs    *ExternalDocs
	security        []SecurityRequirement
	securityPublic  bool
	request         any
	requests        []ContentUnit
	successBody     any
	enveloped       bool
	inType          reflect.Type
	outType         reflect.Type
	errCases        []ErrCase
	extra           []ResponseDecl
}

func routeToContract(route Route) routeContract {
	return routeContract{
		responses:       route.Responses,
		explicitRequest: route.Request,
		summary:         route.Summary,
		description:     route.Description,
		operationID:     route.OperationID,
		tags:            append([]string(nil), route.Tags...),
		specs:           append([]string(nil), route.Specs...),
		deprecated:      route.Deprecated,
		hidden:          route.Hidden,
		externalDocs:    route.ExternalDocs,
		requests:        append([]ContentUnit(nil), route.Requests...),
		extra:           append([]ResponseDecl(nil), route.Extra...),
	}
}

func groupToContract(g GroupConfig) routeContract {
	return routeContract{
		responses:   g.Responses,
		summary:     g.Summary,
		description: g.Description,
		tags:        append([]string(nil), g.Tags...),
		specs:       append([]string(nil), g.Specs...),
		deprecated:  g.Deprecated,
		hidden:      g.Hidden,
	}
}

func mergeContract(base, child routeContract) routeContract {
	out := routeContract{
		summary:     base.summary,
		description: base.description,
		operationID: base.operationID,
		deprecated:  base.deprecated || child.deprecated,
		hidden:      base.hidden || child.hidden,
		tags:        appendDedupe(base.tags, child.tags),
		specs:       appendDedupe(base.specs, child.specs),
		responses:   base.responses,
		requests:    base.requests,
		extra:       base.extra,
	}
	if child.summary != "" {
		out.summary = child.summary
	}
	if child.description != "" {
		out.description = child.description
	}
	if child.operationID != "" {
		out.operationID = child.operationID
	}
	if child.externalDocs != nil {
		out.externalDocs = child.externalDocs
	}
	if len(child.requests) > 0 {
		out.requests = child.requests
	}
	if child.responses != nil {
		out.responses = child.responses
	}
	if child.explicitRequest != nil {
		out.explicitRequest = child.explicitRequest
	}
	return out
}

// Router registers typed routes and records manifest entries for OpenAPI.
type Router struct {
	router  fiber.Router
	catalog *errors.Catalog
	base    routeContract

	kindStatuses    KindStatuses
	defaultStatus   int
	prefix          string
	rec             *recorder
	schemes         map[string]Scheme
	defaultSecurity SecuritySpec
	securityStack   []SecuritySpec
}

func newRouter(
	r fiber.Router,
	catalog *errors.Catalog,
	kindStatuses KindStatuses,
	defaultStatus int,
	prefix string,
	rec *recorder,
	schemes map[string]Scheme,
	defaultSecurity SecuritySpec,
) *Router {
	return &Router{
		router:          r,
		catalog:         catalog,
		kindStatuses:    kindStatuses,
		defaultStatus:   defaultStatus,
		prefix:          prefix,
		rec:             rec,
		schemes:         schemes,
		defaultSecurity: defaultSecurity,
	}
}

// Group creates a sub-router under g.Prefix.
func (r *Router) Group(g GroupConfig) *Router {
	stack := r.securityStack
	if g.Security.isExplicit() {
		stack = append(append([]SecuritySpec(nil), stack...), g.Security)
	}
	return &Router{
		router:          r.router.Group(g.Prefix),
		catalog:         r.catalog,
		base:            mergeContract(r.base, groupToContract(g)),
		kindStatuses:    r.kindStatuses,
		defaultStatus:   r.defaultStatus,
		prefix:          joinPath(r.prefix, g.Prefix),
		rec:             r.rec,
		schemes:         r.schemes,
		defaultSecurity: r.defaultSecurity,
		securityStack:   stack,
	}
}

// Fiber returns the underlying fiber.Router for non-openapi registration.
func (r *Router) Fiber() fiber.Router {
	return r.router
}

func (r *Router) record(method, path string, re routeContract, compiled *compile.Route) {
	specs := errorSpecsFromCases(re.errCases)
	sort.Slice(specs, func(i, j int) bool { return specs[i].Code < specs[j].Code })

	full := joinPath(r.prefix, path)
	documented := re.summary != "" || re.description != "" || re.operationID != "" ||
		re.explicitRequest != nil || len(re.requests) > 0 || len(re.extra) > 0 ||
		responsesDocumented(re.responses)

	r.rec.add(RouteSpec{
		Method:      method,
		Path:        full,
		Summary:     re.summary,
		Description: re.description,
		OperationID: re.operationID,
		Tags:        re.tags,
		Specs:       re.specs,
		Params:      pathParams(full),
		Errors:      specs,
		Documented:  documented,
		op: operationDecl{
			deprecated:     re.deprecated,
			hidden:         re.hidden,
			externalDocs:   re.externalDocs,
			security:       re.security,
			securityPublic: re.securityPublic,
			request:        re.request,
			requests:       re.requests,
			successBody:    re.successBody,
			enveloped:      re.enveloped,
			inType:         re.inType,
			outType:        re.outType,
			errCases:       re.errCases,
			extra:          re.extra,
		},
	})
}

func responsesDocumented(rs *ResponseSet) bool {
	if rs == nil {
		return false
	}
	if rs.SuccessBody() != nil {
		return true
	}
	return len(rs.Cases()) > 0
}

func errorSpecsFromCases(cases []ErrCase) []ErrorSpec {
	seen := map[*errors.Error]bool{}
	var specs []ErrorSpec
	for _, c := range cases {
		for _, e := range c.Errors {
			if seen[e] {
				continue
			}
			seen[e] = true
			specs = append(specs, errorSpecFrom(e, c.Status))
		}
	}
	return specs
}

type operationDecl struct {
	deprecated     bool
	hidden         bool
	externalDocs   *ExternalDocs
	security       []SecurityRequirement
	securityPublic bool
	request        any
	requests       []ContentUnit
	successBody    any
	enveloped      bool
	inType         reflect.Type
	outType        reflect.Type
	errCases       []ErrCase
	extra          []ResponseDecl
}
