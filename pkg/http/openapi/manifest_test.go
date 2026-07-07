package openapi_test

import (
	"context"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
)

var manifestCatalog = errors.NewCatalog("manifest")

var (
	manValidation = manifestCatalog.Define(errors.Def{Kind: errors.KindValidation, Code: "10001", Message: "validation failed"})
	manNotFound   = manifestCatalog.Define(errors.Def{Kind: errors.KindNotFound, Code: "10002", Message: "not found"})
	manUnhandled  = manifestCatalog.Define(errors.Def{Kind: errors.KindInternal, Code: "10500", Message: "internal error"})
	manRouteMiss  = manifestCatalog.Define(errors.Def{Kind: errors.KindNotFound, Code: "10404", Message: "route not found"})
)

type manIn struct {
	ID int64 `path:"id"`
}

type manOut struct {
	openapi.Response
}

func manOK(_ context.Context, _ manIn) (manOut, error) {
	return manOut{}, nil
}

func newManifestAPI() *openapi.Generator {
	_, api := newAPI(httpConfigWithCatalog(manifestCatalog))
	return api
}

func httpConfigWithCatalog(catalog *errors.Catalog) http.Config {
	cfg := routerHTTPConfig()
	cfg.Catalog = catalog
	if catalog == manifestCatalog {
		cfg.Fallbacks = http.Fallbacks{
			Unhandled: manUnhandled,
			NotFound:  manRouteMiss,
		}
	}
	return cfg
}

func TestManifestRecordsRoute(t *testing.T) {
	api := newManifestAPI()
	api.Routes("/api", func(r *openapi.Router) {
		r.Route("/users/:id").Get(openapi.Route{
			Handler:   manOK,
			Summary:   "Fetch user",
			Responses: openapi.Returns().Err(422, manValidation).Err(404, manNotFound),
		})
	})

	m := api.Manifest()
	if len(m.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(m.Routes))
	}
	route := m.Routes[0]
	if route.Method != "GET" || route.Path != "/api/users/:id" {
		t.Fatalf("unexpected route: %+v", route)
	}
	if !route.Documented {
		t.Fatal("expected documented route")
	}
	if len(route.Errors) != 2 {
		t.Fatalf("unexpected errors: %+v", route.Errors)
	}
}

func bareOK(_ context.Context, _ struct{}) (manOut, error) {
	return manOut{}, nil
}

func TestGroupInheritsResponses(t *testing.T) {
	api := newManifestAPI()
	api.Routes("/api", func(r *openapi.Router) {
		v1 := r.Group(openapi.GroupConfig{
			Prefix:    "/v1",
			Responses: openapi.Returns().Err(422, manValidation),
			Tags:      []string{"v1"},
		})
		v1.Route("/users/:id").Get(openapi.Route{
			Handler:   manOK,
			Summary:   "Fetch",
			Responses: openapi.Returns().Err(404, manNotFound),
		})
	})

	route := mRoute(t, api.Manifest(), "GET", "/api/v1/users/:id")
	codes := map[string]int{}
	for _, e := range route.Errors {
		codes[e.Code] = e.HTTPStatus
	}
	if codes["10001"] != 422 || codes["10002"] != 404 {
		t.Fatalf("expected merged errors, got %+v", route.Errors)
	}
}

func TestUndocumentedBareHandler(t *testing.T) {
	api := newManifestAPI()
	api.Routes("/", func(r *openapi.Router) {
		r.Route("/internal").Get(openapi.Route{
			Handler:   bareOK,
			Responses: openapi.Returns(),
		})
	})

	route := mRoute(t, api.Manifest(), "GET", "/internal")
	if route.Documented {
		t.Fatal("bare handler should not be documented")
	}
}

func mRoute(t *testing.T, m openapi.Manifest, method, path string) openapi.RouteSpec {
	t.Helper()
	for _, r := range m.Routes {
		if r.Method == method && r.Path == path {
			return r
		}
	}
	t.Fatalf("route %s %s not found", method, path)
	return openapi.RouteSpec{}
}
