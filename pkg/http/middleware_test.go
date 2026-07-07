package http_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/logger"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var mwTestCatalog = errors.NewCatalog("middleware")

var (
	errMWUnhandled = mwTestCatalog.Define(errors.Def{
		Code: "500000", Message: "unhandled",
	})
	errMWNotFound = mwTestCatalog.Define(errors.Def{
		Code: "404000", Message: "not found",
	})
)

func testHTTPConfig() http.Config {
	log := logger.New(&logger.Config{Name: "test", ID: "test-1"})
	return http.Config{
		Host:          "127.0.0.1",
		Port:          8080,
		Logger:        log,
		Catalog:       mwTestCatalog,
		DefaultStatus: 500,
		Fallbacks: http.Fallbacks{
			Unhandled: errMWUnhandled,
			NotFound:  errMWNotFound,
		},
	}
}

func TestRequestIDInEnvelope(t *testing.T) {
	tr := http.New(testHTTPConfig())
	tr.Fiber().Get("/", func(c http.Ctx) error { return http.RES(c).Message("ok").Ok() })

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}

	id, ok := out["request_id"].(string)
	if !ok || id == "" {
		t.Fatalf("expected non-empty request_id, got %#v", out["request_id"])
	}
	if hdr := resp.Header.Get("X-Request-Id"); hdr != id {
		t.Fatalf("expected X-Request-Id header %q to match envelope request_id, got %q", id, hdr)
	}
}

func TestClientRequestIDEcho(t *testing.T) {
	tr := http.New(testHTTPConfig())
	tr.Fiber().Get("/", func(c http.Ctx) error { return http.RES(c).Message("ok").Ok() })

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(http.HeaderClientRequestID, "client-abc-123")

	resp, err := tr.Fiber().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if hdr := resp.Header.Get(http.HeaderClientRequestID); hdr != "client-abc-123" {
		t.Fatalf("expected client request id echoed in header, got %q", hdr)
	}

	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out["client_request_id"] != "client-abc-123" {
		t.Fatalf("expected client_request_id in envelope, got %#v", out["client_request_id"])
	}
}

func TestClientRequestIDAbsentOrInvalid(t *testing.T) {
	tr := http.New(testHTTPConfig())
	tr.Fiber().Get("/", func(c http.Ctx) error { return http.RES(c).Message("ok").Ok() })

	t.Run("absent header is not reflected", func(t *testing.T) {
		resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if hdr := resp.Header.Get(http.HeaderClientRequestID); hdr != "" {
			t.Fatalf("expected no client request id header, got %q", hdr)
		}
		body, _ := io.ReadAll(resp.Body)
		var out map[string]any
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		if _, ok := out["client_request_id"]; ok {
			t.Fatal("client_request_id should be omitted when not supplied")
		}
	})

	t.Run("oversized header is rejected", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(http.HeaderClientRequestID, strings.Repeat("a", 200))

		resp, err := tr.Fiber().Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if hdr := resp.Header.Get(http.HeaderClientRequestID); hdr != "" {
			t.Fatalf("expected oversized client request id to be dropped, got %q", hdr)
		}
	})
}

func TestAccessLogUsesErrorStatus(t *testing.T) {
	core, recorded := observer.New(zapcore.ErrorLevel)
	log := logger.FromZap(&logger.Config{Name: "test", ID: "test-1"}, zap.New(core))

	cfg := testHTTPConfig()
	cfg.Logger = log
	tr := http.New(cfg)
	tr.Fiber().Get("/boom", func(c http.Ctx) error { return errors.NewPlain("simulated failure") })

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/boom", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	entries := recorded.AllUntimed()
	var accessLine string
	for _, e := range entries {
		if strings.Contains(e.Message, "GET. /boom.") {
			accessLine = e.Message
			break
		}
	}
	if accessLine == "" {
		t.Fatalf("expected access log for /boom, got %d entries", len(entries))
	}
	if !strings.Contains(accessLine, "status=500") {
		t.Fatalf("expected access log status=500, got %q", accessLine)
	}
}

func TestAccessLogUses4xxStatus(t *testing.T) {
	core, recorded := observer.New(zapcore.WarnLevel)
	log := logger.FromZap(&logger.Config{Name: "test", ID: "test-1"}, zap.New(core))

	cfg := testHTTPConfig()
	cfg.Logger = log
	tr := http.New(cfg)
	tr.Fiber().Get("/bad", func(c http.Ctx) error {
		return fiber.NewError(fiber.StatusUnprocessableEntity, "validation failed")
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/bad", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}

	entries := recorded.AllUntimed()
	var accessLine string
	for _, e := range entries {
		if strings.Contains(e.Message, "GET. /bad.") {
			accessLine = e.Message
			break
		}
	}
	if accessLine == "" {
		t.Fatalf("expected access log for /bad, got %d entries", len(entries))
	}
	if !strings.Contains(accessLine, "status=422") {
		t.Fatalf("expected access log status=422, got %q", accessLine)
	}
}

func TestErrorHandlerPlainError(t *testing.T) {
	tr := http.New(testHTTPConfig())
	tr.Fiber().Get("/boom", func(c http.Ctx) error { return errors.NewPlain("simulated failure") })

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/boom", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Status string             `json:"status"`
		Errors []http.ErrorDetail `json:"errors"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "error" || len(out.Errors) != 1 || out.Errors[0].Code != "500000" {
		t.Fatalf("unexpected envelope: %+v", out)
	}
}

func TestErrorHandlerNotFoundRoute(t *testing.T) {
	tr := http.New(testHTTPConfig())

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/missing", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Status string             `json:"status"`
		Errors []http.ErrorDetail `json:"errors"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out.Errors[0].Code != "404000" {
		t.Fatalf("unexpected error code: %+v", out.Errors)
	}
}

func TestNewPanicsWithoutCatalog(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic without catalog")
		}
	}()

	log := logger.New(&logger.Config{Name: "test", ID: "test-1"})
	http.New(http.Config{Logger: log})
}

func TestNewAppliesDefaults(t *testing.T) {
	log := logger.New(&logger.Config{Name: "test", ID: "test-1"})
	cat := errors.NewCatalog("defaults")

	tr := http.New(http.Config{Logger: log, Catalog: cat})
	if tr.Name() == "" {
		t.Fatal("expected transport name")
	}

	// Partial Fallbacks still panic — only the all-zero case is defaulted.
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for partial fallbacks")
		}
	}()
	http.New(http.Config{
		Logger:  log,
		Catalog: cat,
		Fallbacks: http.Fallbacks{
			Unhandled: cat.Define(errors.Def{Kind: errors.KindInternal, Code: "x", Message: "x"}),
		},
	})
}
