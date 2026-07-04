package http_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/http"

	"github.com/gofiber/fiber/v3"
)

var routerCatalog = errors.NewCatalog("router")

var (
	rtNotFound   = routerCatalog.Define(errors.Def{Kind: "not_found", Code: "10001", Message: "not found"})
	rtValidation = routerCatalog.Define(errors.Def{Kind: "validation", Code: "10002", Message: "invalid input"})
	rtInternal   = routerCatalog.Define(errors.Def{Kind: "internal", Code: "10003", Message: "boom"})
	rtUnmapped   = routerCatalog.Define(errors.Def{Code: "10004", Message: "unmapped"})
	rtUnhandled  = routerCatalog.Define(errors.Def{Code: "10500", Message: "internal error"})
	rtRouteMiss  = routerCatalog.Define(errors.Def{Code: "10404", Message: "route not found"})
)

// internalCatalog simulates a module-level catalog that must not leak.
var internalCatalog = errors.NewCatalog("store")

var internalErr = internalCatalog.Define(errors.Def{Code: "10001", Message: "internal store failure"})

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

func TestRouteMapsErrorToStatus(t *testing.T) {
	tr := http.New(routerHTTPConfig()).Routes("/", func(r *http.Router) {
		r.Get("/users/:id", func(c fiber.Ctx) error {
			return rtNotFound
		}, http.Status(rtNotFound, 404))
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
	tr := http.New(routerHTTPConfig()).Routes("/", func(r *http.Router) {
		r.Post("/users", func(c fiber.Ctx) error {
			return errors.Errors{rtValidation, rtInternal}
		}, http.Statuses(http.StatusMap{
			rtValidation: 422,
			rtInternal:   500,
		}))
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

func TestRouteUnmappedBubblesToGlobal(t *testing.T) {
	tr := http.New(routerHTTPConfig()).Routes("/", func(r *http.Router) {
		r.Get("/x", func(c fiber.Ctx) error {
			return rtUnmapped
		}, http.Status(rtNotFound, 404))
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/x", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Unmapped catalog errors reach the global handler and use DefaultStatus.
	if resp.StatusCode != 500 {
		t.Fatalf("expected default 500, got %d", resp.StatusCode)
	}
	if out := readEnvelope(t, resp.Body); out.Errors[0].Code != "10004" {
		t.Fatalf("expected unmapped error preserved, got %+v", out)
	}
}

func TestRouteOnErrorCustom(t *testing.T) {
	tr := http.New(routerHTTPConfig()).Routes("/", func(r *http.Router) {
		r.Get("/x", func(c fiber.Ctx) error {
			return rtInternal
		}, http.OnError(func(c fiber.Ctx, err error) error {
			return http.RES(c).Message("handled").Apply(err).Status(418).Ok()
		}))
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/x", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 418 {
		t.Fatalf("expected custom 418, got %d", resp.StatusCode)
	}
}

func TestGroupInheritsStatuses(t *testing.T) {
	tr := http.New(routerHTTPConfig()).Routes("/api", func(r *http.Router) {
		v1 := r.Group("/v1", http.Status(rtNotFound, 404))
		v1.Get("/users/:id", func(c fiber.Ctx) error {
			return rtNotFound
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

func TestForeignStatusMapPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for status map entry outside the public catalog")
		}
	}()

	http.New(routerHTTPConfig()).Routes("/", func(r *http.Router) {
		r.Get("/x", func(c fiber.Ctx) error { return nil },
			http.Status(internalErr, 404))
	})
}

func TestInternalCatalogErrorDoesNotLeak(t *testing.T) {
	tr := http.New(routerHTTPConfig()).Routes("/", func(r *http.Router) {
		r.Get("/leak", func(c fiber.Ctx) error {
			return internalErr
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
	if len(out.Errors) != 1 || out.Errors[0].Code != rtUnhandled.Code || out.Errors[0].Kind != rtUnhandled.Kind {
		t.Fatalf("expected only the Unhandled fallback in the body, got %+v", out.Errors)
	}
	if out.Errors[0].Message == internalErr.Message {
		t.Fatal("internal error details must not leak to the wire")
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
