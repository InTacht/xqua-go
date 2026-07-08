package security_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/compile"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/security"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/wire"

	"github.com/gofiber/fiber/v3"
)

var guardCatalog = errors.NewCatalog("guard")

var (
	errUnauthorized  = guardCatalog.Define(errors.Def{Kind: errors.KindUnauthorized, Code: "401001", Message: "missing bearer"})
	errExpired       = guardCatalog.Define(errors.Def{Kind: errors.KindUnauthorized, Code: "401002", Message: "api key expired"})
	errInvalidAPIKey = guardCatalog.Define(errors.Def{Kind: errors.KindUnauthorized, Code: "401003", Message: "invalid api key"})
	errForbidden     = guardCatalog.Define(errors.Def{Kind: errors.KindForbidden, Code: "14003", Message: "forbidden"})
)

func TestGuardBearerSuccess(t *testing.T) {
	route := &compile.Route{
		ErrCases: []compile.ErrCase{
			{Status: 401, Errors: []*errors.Error{errUnauthorized}},
			{Status: 403, Errors: []*errors.Error{errForbidden}},
		},
		Unauthorized: errUnauthorized,
	}
	schemes := map[string]security.Scheme{
		"Bearer": {
			Extract: func(c fiber.Ctx) (string, bool) {
				auth := c.Get("Authorization")
				if len(auth) < 8 {
					return "", false
				}
				return auth[7:], true
			},
			Verify: func(ctx context.Context, cred security.Credential) (any, error) {
				if cred.Raw != "secret" {
					return nil, errUnauthorized
				}
				return "user-1", nil
			},
		},
	}
	var seen any
	next := func(c fiber.Ctx) error {
		seen, _ = wire.IdentityFrom(c.Context())
		return c.SendStatus(http.StatusOK)
	}
	// use wire through guard - guard uses wire.WithIdentity
	app := fiber.New()
	app.Get("/", security.Guard(route, []security.Requirement{{Names: []string{"Bearer"}}}, schemes, guardCatalog, errUnauthorized, next))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if seen != "user-1" {
		t.Fatalf("expected identity in handler, got %v", seen)
	}
}

func TestGuardMissingCredential401(t *testing.T) {
	route := &compile.Route{
		ErrCases:     []compile.ErrCase{{Status: 401, Errors: []*errors.Error{errUnauthorized}}},
		Unauthorized: errUnauthorized,
	}
	app := fiber.New()
	app.Get("/", security.Guard(route, []security.Requirement{{Names: []string{"Bearer"}}}, map[string]security.Scheme{
		"Bearer": {
			Extract: func(c fiber.Ctx) (string, bool) { return "", false },
			Verify:  func(ctx context.Context, cred security.Credential) (any, error) { return nil, nil },
		},
	}, guardCatalog, errUnauthorized, func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	}))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	if env["status"] != "error" {
		t.Fatalf("expected error envelope, got %v", env)
	}
}

func TestGuardForbiddenScope403(t *testing.T) {
	route := &compile.Route{
		ErrCases: []compile.ErrCase{
			{Status: 401, Errors: []*errors.Error{errUnauthorized}},
			{Status: 403, Errors: []*errors.Error{errForbidden}},
		},
		Unauthorized: errUnauthorized,
	}
	app := fiber.New()
	app.Get("/", security.Guard(route, []security.Requirement{{Names: []string{"Bearer"}, Scopes: []string{"admin"}}}, map[string]security.Scheme{
		"Bearer": {
			Extract: func(c fiber.Ctx) (string, bool) {
				return "token", true
			},
			Verify: func(ctx context.Context, cred security.Credential) (any, error) {
				if len(cred.Scopes) > 0 {
					return nil, errForbidden
				}
				return "user-1", nil
			},
		},
	}, guardCatalog, errUnauthorized, func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	errs, _ := env["errors"].([]any)
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %v", env)
	}
	first, _ := errs[0].(map[string]any)
	if first["code"] != errForbidden.Code {
		t.Fatalf("expected code %q, got %v", errForbidden.Code, first["code"])
	}
}

func TestGuardVerifyReturnsDeclared401NotGeneric(t *testing.T) {
	route := &compile.Route{
		ErrCases: []compile.ErrCase{
			{Status: 401, Errors: []*errors.Error{errUnauthorized, errExpired, errInvalidAPIKey}},
		},
		Unauthorized: errUnauthorized,
	}
	schemes := map[string]security.Scheme{
		"Bearer": {
			Extract: func(c fiber.Ctx) (string, bool) {
				return "present", true
			},
			Verify: func(ctx context.Context, cred security.Credential) (any, error) {
				switch cred.Raw {
				case "expired":
					return nil, errExpired
				case "invalid":
					return nil, errInvalidAPIKey
				default:
					return "user-1", nil
				}
			},
		},
	}
	next := func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) }

	t.Run("expired", func(t *testing.T) {
		app := fiber.New()
		app.Get("/", security.Guard(route, []security.Requirement{{Names: []string{"Bearer"}}}, map[string]security.Scheme{
			"Bearer": {
				Extract: func(c fiber.Ctx) (string, bool) { return "expired", true },
				Verify:  schemes["Bearer"].Verify,
			},
		}, guardCatalog, errUnauthorized, next))
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		assertErrorCode(t, resp, "401002")
	})

	t.Run("invalid", func(t *testing.T) {
		app := fiber.New()
		app.Get("/invalid", security.Guard(route, []security.Requirement{{Names: []string{"Bearer"}}}, map[string]security.Scheme{
			"Bearer": {
				Extract: func(c fiber.Ctx) (string, bool) { return "invalid", true },
				Verify:  schemes["Bearer"].Verify,
			},
		}, guardCatalog, errUnauthorized, next))
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/invalid", nil))
		if err != nil {
			t.Fatal(err)
		}
		assertErrorCode(t, resp, "401003")
	})
}

func assertErrorCode(t *testing.T, resp *http.Response, wantCode string) {
	t.Helper()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	errs, _ := env["errors"].([]any)
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %v", env)
	}
	first, _ := errs[0].(map[string]any)
	if first["code"] != wantCode {
		t.Fatalf("expected code %q, got %v", wantCode, first["code"])
	}
}

func TestValidateRouteRequires401(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing 401")
		}
	}()
	security.ValidateRoute("test", []security.Requirement{{Names: []string{"Bearer"}}}, nil, false)
}
