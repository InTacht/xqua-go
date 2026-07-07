package openapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/logger"
)

var routerCatalog = errors.NewCatalog("router")

var (
	rtNotFound   = routerCatalog.Define(errors.Def{Kind: errors.KindNotFound, Code: "10001", Message: "not found"})
	rtValidation = routerCatalog.Define(errors.Def{Kind: errors.KindValidation, Code: "10002", Message: "invalid input"})
	rtIDInvalid  = routerCatalog.Define(errors.Def{Kind: errors.KindValidation, Code: "10006", Message: "id is invalid", Source: "params.id"})
	rtInternal   = routerCatalog.Define(errors.Def{Kind: errors.KindInternal, Code: "10003", Message: "boom"})
	rtUnmapped   = routerCatalog.Define(errors.Def{Kind: errors.KindInternal, Code: "10004", Message: "unmapped"})
	rtUnhandled  = routerCatalog.Define(errors.Def{Kind: errors.KindInternal, Code: "10500", Message: "internal error"})
	rtRouteMiss  = routerCatalog.Define(errors.Def{Kind: errors.KindNotFound, Code: "10404", Message: "route not found"})
)

var internalCatalog = errors.NewCatalog("store")

var internalErr = internalCatalog.Define(errors.Def{Code: "10001", Message: "internal store failure"})

type getUserIn struct {
	ID int64 `path:"id"`
}

type ackOut struct {
	openapi.Response
}

func routerHTTPConfig() http.Config {
	log := logger.New(&logger.Config{Name: "router-test", ID: "router-test-1"})
	return http.Config{
		Host:          "127.0.0.1",
		Port:          8080,
		Logger:        log,
		Catalog:       routerCatalog,
		DefaultStatus: 500,
		Fallbacks: http.Fallbacks{
			Unhandled: rtUnhandled,
			NotFound:  rtRouteMiss,
		},
	}
}

func newAPI(cfg http.Config) (*http.Transport, *openapi.Generator) {
	tr := http.New(cfg)
	return tr, openapi.New(tr, openapi.Config{Specs: []openapi.Spec{}})
}

type envelopeOut struct {
	Status string             `json:"status"`
	Errors []http.ErrorDetail `json:"errors"`
}

func readEnvelope(t *testing.T, body io.Reader) envelopeOut {
	t.Helper()
	raw, _ := io.ReadAll(body)
	var out envelopeOut
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return out
}

func getUserNotFound(_ context.Context, _ getUserIn) (ackOut, error) {
	return ackOut{}, rtNotFound
}

func postUsersMultiErr(_ context.Context, _ struct{}) (ackOut, error) {
	return ackOut{}, errors.Errors{rtValidation, rtInternal}
}

func getUnmapped(_ context.Context, _ struct{}) (ackOut, error) {
	return ackOut{}, rtUnmapped
}

func TestRouteMapsErrorToStatus(t *testing.T) {
	tr, api := newAPI(routerHTTPConfig())
	api.Routes("/", func(r *openapi.Router) {
		r.Route("/users/:id").Get(openapi.Route{
			Handler: getUserNotFound,
			Responses: openapi.Returns().
				Err(422, rtValidation).
				Err(404, rtNotFound),
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/users/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	out := readEnvelope(t, resp.Body)
	if out.Status != "error" || len(out.Errors) != 1 || out.Errors[0].Code != "10001" {
		t.Fatalf("unexpected envelope: %+v", out)
	}
}

func TestRouteHighestSeverity(t *testing.T) {
	tr, api := newAPI(routerHTTPConfig())
	api.Routes("/", func(r *openapi.Router) {
		r.Route("/users").Post(openapi.Route{
			Handler: postUsersMultiErr,
			Responses: openapi.Returns().
				Err(422, rtValidation).
				Err(500, rtInternal),
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("POST", "/users", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Fatalf("expected highest severity 500, got %d", resp.StatusCode)
	}
	if out := readEnvelope(t, resp.Body); len(out.Errors) != 2 {
		t.Fatalf("expected both errors in body, got %+v", out)
	}
}

func TestRouteUndeclaredBubblesToGlobal(t *testing.T) {
	tr, api := newAPI(routerHTTPConfig())
	api.Routes("/", func(r *openapi.Router) {
		r.Route("/x").Get(openapi.Route{
			Handler:   getUnmapped,
			Responses: openapi.Returns().Err(404, rtNotFound),
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/x", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Fatalf("expected default 500, got %d", resp.StatusCode)
	}
	if out := readEnvelope(t, resp.Body); out.Errors[0].Code != "10004" {
		t.Fatalf("expected unmapped error preserved, got %+v", out)
	}
}

func TestGroupInheritsErrors(t *testing.T) {
	tr, api := newAPI(routerHTTPConfig())
	api.Routes("/api", func(r *openapi.Router) {
		v1 := r.Group(openapi.GroupConfig{
			Prefix:    "/v1",
			Responses: openapi.Returns().Err(422, rtValidation).Err(404, rtNotFound),
		})
		v1.Route("/users/:id").Get(openapi.Route{
			Handler:   getUserNotFound,
			Responses: openapi.Returns(),
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/api/v1/users/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("expected inherited 404, got %d", resp.StatusCode)
	}
}

func TestForeignErrPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for error outside the public catalog")
		}
	}()

	_, api := newAPI(routerHTTPConfig())
	api.Routes("/", func(r *openapi.Router) {
		r.Route("/x").Get(openapi.Route{
			Handler:   func(context.Context, struct{}) (ackOut, error) { return ackOut{}, nil },
			Responses: openapi.Returns().Err(404, internalErr),
		})
	})
}

func TestInternalCatalogErrorDoesNotLeak(t *testing.T) {
	tr, api := newAPI(routerHTTPConfig())
	api.Routes("/", func(r *openapi.Router) {
		r.Route("/leak").Get(openapi.Route{
			Handler: func(context.Context, struct{}) (ackOut, error) {
				return ackOut{}, internalErr
			},
			Responses: openapi.Returns(),
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/leak", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Fatalf("expected default 500, got %d", resp.StatusCode)
	}

	out := readEnvelope(t, resp.Body)
	if len(out.Errors) != 1 || out.Errors[0].Code != rtUnhandled.Code {
		t.Fatalf("expected only the Unhandled fallback in the body, got %+v", out.Errors)
	}
}

func TestFallbacksMustBelongToCatalog(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for fallbacks outside the public catalog")
		}
	}()

	cfg := routerHTTPConfig()
	cfg.Fallbacks.Unhandled = internalErr
	http.New(cfg)
}

func TestRouteHandlerRequired(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when Handler is nil")
		}
	}()
	_, api := newAPI(routerHTTPConfig())
	api.Routes("/", func(r *openapi.Router) {
		r.Route("/x").Get(openapi.Route{})
	})
}

func TestEnvelopedSuccess(t *testing.T) {
	tr, api := newAPI(routerHTTPConfig())
	api.Routes("/", func(r *openapi.Router) {
		r.Route("/ok").Get(openapi.Route{
			Handler: func(_ context.Context, _ struct{}) (ackOut, error) {
				return ackOut{Response: openapi.Response{Message: "done"}}, nil
			},
			Responses: openapi.Returns(),
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/ok", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "success" || out.Message != "done" {
		t.Fatalf("unexpected body: %+v", out)
	}
}

func TestBindFailureUsesGroupValidation(t *testing.T) {
	tr, api := newAPI(routerHTTPConfig())
	api.Routes("/api", func(r *openapi.Router) {
		v1 := r.Group(openapi.GroupConfig{
			Prefix:    "/v1",
			Responses: openapi.Returns().Err(422, rtValidation),
		})
		v1.Route("/users/:id").Get(openapi.Route{
			Handler:   func(_ context.Context, _ getUserIn) (ackOut, error) { return ackOut{}, nil },
			Responses: openapi.Returns(),
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/api/v1/users/nope", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
	out := readEnvelope(t, resp.Body)
	if out.Status != "error" || len(out.Errors) != 1 || out.Errors[0].Code != rtValidation.Code {
		t.Fatalf("unexpected envelope: %+v", out)
	}
}

func TestBindFailureUsesSourceSpecificCatalog(t *testing.T) {
	tr, api := newAPI(routerHTTPConfig())
	api.Routes("/", func(r *openapi.Router) {
		r.Route("/users/:id").Get(openapi.Route{
			Handler:   func(_ context.Context, _ getUserIn) (ackOut, error) { return ackOut{}, nil },
			Responses: openapi.Returns().Err(422, rtIDInvalid),
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/users/nope", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
	out := readEnvelope(t, resp.Body)
	if len(out.Errors) != 1 || out.Errors[0].Code != rtIDInvalid.Code {
		t.Fatalf("expected source-specific catalog code, got %+v", out.Errors)
	}
	if out.Errors[0].Source != "params.id" {
		t.Fatalf("expected source params.id, got %q", out.Errors[0].Source)
	}
}
