package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"

	"github.com/gofiber/fiber/v3"
)

// fullEnvelope decodes the complete response envelope for the feature tests.
type fullEnvelope struct {
	Status  string             `json:"status"`
	Message string             `json:"message"`
	Data    map[string]any     `json:"data"`
	Errors  []http.ErrorDetail `json:"errors"`
}

func readFull(t *testing.T, body io.Reader) fullEnvelope {
	t.Helper()
	raw, _ := io.ReadAll(body)
	var out fullEnvelope
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode envelope: %v (%s)", err, raw)
	}
	return out
}

func TestKindStatusResolution(t *testing.T) {
	tr := http.New(routerHTTPConfig()).Routes("/", func(r *http.Router) {
		// no explicit status: resolves via kind table (not_found → 404)
		r.Get("/kind-notfound", func(c fiber.Ctx) error { return rtNotFound })
		// unknown kind ("router" catalog name): falls back to DefaultStatus (500)
		r.Get("/kind-unknown", func(c fiber.Ctx) error { return rtUnmapped })
		// explicit override wins over the kind default
		r.Get("/override", func(c fiber.Ctx) error { return rtNotFound },
			http.Status(rtNotFound, 499))
		// highest resolved status across a collection (validation 422 > not_found 404)
		r.Get("/highest", func(c fiber.Ctx) error {
			return errors.Errors{rtNotFound, rtValidation}
		})
	})

	cases := []struct {
		path string
		want int
	}{
		{"/kind-notfound", 404},
		{"/kind-unknown", 500},
		{"/override", 499},
		{"/highest", 422},
	}
	for _, tc := range cases {
		resp, err := tr.Fiber().Test(httptest.NewRequest("GET", tc.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: expected %d, got %d", tc.path, tc.want, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestKindStatusesOverridable(t *testing.T) {
	cfg := routerHTTPConfig()
	cfg.KindStatuses = http.DefaultKindStatuses()
	cfg.KindStatuses[errors.KindNotFound] = 418 // teapot for not_found

	tr := http.New(cfg).Routes("/", func(r *http.Router) {
		r.Get("/x", func(c fiber.Ctx) error { return rtNotFound })
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/x", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 418 {
		t.Fatalf("expected overridden 418, got %d", resp.StatusCode)
	}
}

func TestStandardErrors(t *testing.T) {
	cat := errors.NewCatalog("standard-errors-test")
	fb := http.StandardErrors(cat)

	if fb.Unhandled == nil || fb.NotFound == nil {
		t.Fatal("expected both fallbacks defined")
	}
	if fb.Unhandled.Kind != errors.KindInternal {
		t.Errorf("expected Unhandled kind internal, got %q", fb.Unhandled.Kind)
	}
	if fb.NotFound.Kind != errors.KindNotFound {
		t.Errorf("expected NotFound kind not_found, got %q", fb.NotFound.Kind)
	}
	if !cat.Contains(fb.Unhandled) || !cat.Contains(fb.NotFound) {
		t.Fatal("expected fallbacks to be entries of the catalog")
	}
	// The fallbacks are valid Config.Fallbacks (no panic on New).
	cfg := routerHTTPConfig()
	cfg.Catalog = cat
	cfg.Fallbacks = fb
	http.New(cfg)
}

func TestParamHelpers(t *testing.T) {
	tr := http.New(routerHTTPConfig()).Routes("/", func(r *http.Router) {
		r.Get("/i64/:id", func(c fiber.Ctx) error {
			id, err := http.ParamInt64(c, "id")
			if err != nil {
				if !errors.Is(err, http.ErrInvalidParam) {
					t.Error("expected ErrInvalidParam sentinel")
				}
				return http.RES(c).Message("invalid").Ok()
			}
			return http.RES(c).Message("ok").Data("id", id).Ok()
		})
		r.Get("/int/:n", func(c fiber.Ctx) error {
			n, err := http.ParamInt(c, "n")
			if err != nil {
				return http.RES(c).Message("invalid").Ok()
			}
			return http.RES(c).Message("ok").Data("n", n).Ok()
		})
	})

	t.Run("parses valid int64", func(t *testing.T) {
		resp, _ := tr.Fiber().Test(httptest.NewRequest("GET", "/i64/42", nil))
		defer resp.Body.Close()
		out := readFull(t, resp.Body)
		if out.Message != "ok" || out.Data["id"] != float64(42) {
			t.Fatalf("expected id 42, got %+v", out)
		}
	})

	t.Run("invalid int64 yields sentinel", func(t *testing.T) {
		resp, _ := tr.Fiber().Test(httptest.NewRequest("GET", "/i64/abc", nil))
		defer resp.Body.Close()
		if out := readFull(t, resp.Body); out.Message != "invalid" {
			t.Fatalf("expected invalid, got %+v", out)
		}
	})

	t.Run("parses valid int", func(t *testing.T) {
		resp, _ := tr.Fiber().Test(httptest.NewRequest("GET", "/int/7", nil))
		defer resp.Body.Close()
		if out := readFull(t, resp.Body); out.Data["n"] != float64(7) {
			t.Fatalf("expected n 7, got %+v", out)
		}
	})
}

func TestHealthEndpoint(t *testing.T) {
	t.Run("nil check is always alive", func(t *testing.T) {
		tr := http.New(routerHTTPConfig())
		resp, _ := tr.Fiber().Test(httptest.NewRequest("GET", "/health", nil))
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if out := readFull(t, resp.Body); out.Data["status"] != "alive" {
			t.Fatalf("expected alive, got %+v", out)
		}
	})

	t.Run("healthy check returns 200", func(t *testing.T) {
		cfg := routerHTTPConfig()
		cfg.HealthCheck = func(context.Context) error { return nil }
		tr := http.New(cfg)
		resp, _ := tr.Fiber().Test(httptest.NewRequest("GET", "/health", nil))
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("failing check returns 503", func(t *testing.T) {
		cfg := routerHTTPConfig()
		cfg.HealthCheck = func(context.Context) error { return errors.NewPlain("db down") }
		tr := http.New(cfg)
		resp, _ := tr.Fiber().Test(httptest.NewRequest("GET", "/health", nil))
		defer resp.Body.Close()
		if resp.StatusCode != 503 {
			t.Fatalf("expected 503, got %d", resp.StatusCode)
		}
		out := readFull(t, resp.Body)
		if out.Status != "error" || out.Data["status"] != "unavailable" {
			t.Fatalf("expected error/unavailable envelope, got %+v", out)
		}
	})
}

func TestVersionEndpoint(t *testing.T) {
	cfg := routerHTTPConfig()
	cfg.Version = "1.2.3"
	cfg.BuildID = "abc123"
	tr := http.New(cfg)

	resp, _ := tr.Fiber().Test(httptest.NewRequest("GET", "/version", nil))
	defer resp.Body.Close()
	out := readFull(t, resp.Body)
	if out.Data["version"] != "1.2.3" || out.Data["build_id"] != "abc123" {
		t.Fatalf("unexpected version payload: %+v", out)
	}
}

func TestManifestRecording(t *testing.T) {
	tr := http.New(routerHTTPConfig()).Routes("/api/v1", func(r *http.Router) {
		r.Get("/users/:id", func(c fiber.Ctx) error { return nil },
			http.Status(rtNotFound, 404),
			http.Status(rtValidation, 422),
		)
		r.Get("/plain", func(c fiber.Ctx) error { return nil })
	})

	m := tr.Manifest()
	if m.EnvelopeVersion != http.EnvelopeVersion {
		t.Fatalf("expected envelope version %q, got %q", http.EnvelopeVersion, m.EnvelopeVersion)
	}

	byPath := map[string]http.RouteSpec{}
	for _, r := range m.Routes {
		byPath[r.Method+" "+r.Path] = r
	}

	// ops endpoints are recorded
	if _, ok := byPath["GET /health"]; !ok {
		t.Error("expected /health recorded in manifest")
	}
	if _, ok := byPath["GET /version"]; !ok {
		t.Error("expected /version recorded in manifest")
	}

	users, ok := byPath["GET /api/v1/users/:id"]
	if !ok {
		t.Fatalf("expected users route recorded, got %+v", m.Routes)
	}
	if len(users.Errors) != 2 {
		t.Fatalf("expected 2 declared errors, got %+v", users.Errors)
	}
	statusByCode := map[string]int{}
	for _, e := range users.Errors {
		statusByCode[e.Code] = e.HTTPStatus
	}
	if statusByCode[rtNotFound.Code] != 404 || statusByCode[rtValidation.Code] != 422 {
		t.Fatalf("unexpected resolved statuses: %+v", users.Errors)
	}

	// full public catalog present
	if len(m.Catalog) == 0 {
		t.Fatal("expected catalog entries in manifest")
	}
}
