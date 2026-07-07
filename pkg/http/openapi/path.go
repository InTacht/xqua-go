package openapi

import (
	"reflect"
	"slices"

	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/adapter"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/compile"

	"github.com/gofiber/fiber/v3"
)

// PathRouter registers one or more HTTP methods on a fixed path.
type PathRouter struct {
	parent *Router
	path   string
}

// PathDescribe records documentation-only operations on a fixed path.
type PathDescribe struct {
	parent *Router
	path   string
}

// Route begins registering handlers on path (relative to the router prefix).
func (r *Router) Route(path string) *PathRouter {
	return &PathRouter{parent: r, path: path}
}

// Describe begins recording documentation-only operations on path.
func (r *Router) Describe(path string) *PathDescribe {
	return &PathDescribe{parent: r, path: path}
}

func (p *PathRouter) Get(route Route) *PathRouter    { return p.register(fiber.MethodGet, route) }
func (p *PathRouter) Post(route Route) *PathRouter   { return p.register(fiber.MethodPost, route) }
func (p *PathRouter) Put(route Route) *PathRouter    { return p.register(fiber.MethodPut, route) }
func (p *PathRouter) Patch(route Route) *PathRouter  { return p.register(fiber.MethodPatch, route) }
func (p *PathRouter) Delete(route Route) *PathRouter { return p.register(fiber.MethodDelete, route) }
func (p *PathRouter) Query(route Route) *PathRouter  { return p.register(http.MethodQuery, route) }
func (p *PathRouter) Handle(method string, route Route) *PathRouter {
	return p.register(method, route)
}

func (p *PathRouter) register(method string, route Route) *PathRouter {
	compiled, contract := p.parent.compileRoute(method, p.path, route, false)
	if p.parent.router != nil {
		p.parent.router.Add([]string{method}, p.path, adapter.Handler(compiled, p.parent.catalog))
	}
	p.parent.record(method, p.path, contract, compiled)
	return p
}

func (p *PathDescribe) Get(route Route) *PathDescribe   { return p.describe(fiber.MethodGet, route) }
func (p *PathDescribe) Post(route Route) *PathDescribe  { return p.describe(fiber.MethodPost, route) }
func (p *PathDescribe) Put(route Route) *PathDescribe   { return p.describe(fiber.MethodPut, route) }
func (p *PathDescribe) Patch(route Route) *PathDescribe { return p.describe(fiber.MethodPatch, route) }
func (p *PathDescribe) Delete(route Route) *PathDescribe {
	return p.describe(fiber.MethodDelete, route)
}
func (p *PathDescribe) Query(route Route) *PathDescribe { return p.describe(http.MethodQuery, route) }
func (p *PathDescribe) Handle(method string, route Route) *PathDescribe {
	return p.describe(method, route)
}

func (p *PathDescribe) describe(method string, route Route) *PathDescribe {
	if route.Handler != nil {
		panic("openapi: Describe route must not set Handler")
	}
	compiled, contract := p.parent.compileRoute(method, p.path, route, true)
	p.parent.record(method, p.path, contract, compiled)
	return p
}

func (r *Router) compileRoute(method, path string, route Route, describe bool) (*compile.Route, routeContract) {
	merged := MergeResponseSets(r.base.responses, route.Responses)
	errCases := toCompileErrCases(merged.Cases())

	successType := merged.SuccessType()
	compiled := compile.Build(compile.Input{
		Method:        method,
		Path:          path,
		Describe:      describe,
		Handler:       route.Handler,
		Request:       route.Request,
		InferSuccess:  merged.InferSuccess(),
		SuccessType:   successType,
		ErrCases:      errCases,
		Enveloped:     false,
		Extra:         toCompileExtra(route.Extra),
		Catalog:       r.catalog,
		KindStatuses:  r.kindStatuses,
		DefaultStatus: r.defaultStatus,
	})
	if compiled.SuccessType != nil {
		compiled.Enveloped = Enveloped(compiled.SuccessType)
	}

	re := mergeContract(r.base, routeToContract(route))
	re.responses = merged
	re.request = requestValue(route, compiled.InType)
	re.successBody = successSample(compiled.SuccessType)
	re.enveloped = compiled.Enveloped
	re.inType = compiled.InType
	re.outType = compiled.SuccessType
	re.errCases = merged.Cases()
	re.extra = append([]ResponseDecl(nil), route.Extra...)

	return compiled, re
}

func requestValue(route Route, inType reflect.Type) any {
	if route.Request != nil {
		return route.Request
	}
	if inType == nil {
		return nil
	}
	return reflect.New(inType).Interface()
}

func successSample(outType reflect.Type) any {
	if outType == nil {
		return nil
	}
	return reflect.New(outType).Interface()
}

func toCompileExtra(extra []ResponseDecl) []compile.ResponseDecl {
	out := make([]compile.ResponseDecl, len(extra))
	for i, e := range extra {
		out[i] = compile.ResponseDecl{Status: e.Status, ItemBody: e.ItemBody}
	}
	return out
}

func toCompileErrCases(cases []ErrCase) []compile.ErrCase {
	out := make([]compile.ErrCase, len(cases))
	for i, c := range cases {
		out[i] = compile.ErrCase{Status: c.Status, Errors: c.Errors}
	}
	return out
}

func indirectType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// appendDedupe returns base followed by entries of add not already present.
func appendDedupe[T comparable](base, add []T) []T {
	out := append([]T(nil), base...)
	for _, v := range add {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}
