package openapi_test

import (
	"context"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/gofiber/fiber/v3"
)

var (
	rtRateLimited = routerCatalog.Define(errors.Def{Kind: errors.KindRateLimit, Code: "10429", Message: "rate limited"})
)

func TestAfterAuthWritesDeclaredError(t *testing.T) {
	tr, api := newAPI(routerHTTPConfig())
	var hits atomic.Int32

	api.Routes("/", func(r *openapi.Router) {
		r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().Err(429, rtRateLimited),
			AfterAuth: []openapi.Middleware{
				func(c fiber.Ctx, _ openapi.RouteContext) error {
					if hits.Add(1) > 2 {
						return rtRateLimited
					}
					return nil
				},
			},
		}).Route("/limited").Get(openapi.Route{
			Handler: func(_ context.Context, _ struct{}) (ackOut, error) {
				return ackOut{Response: openapi.Response{Message: "ok"}}, nil
			},
			Summary: "Rate limited demo route",
		})
	})

	for i := range 3 {
		resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/limited", nil))
		if err != nil {
			t.Fatal(err)
		}
		if i < 2 {
			if resp.StatusCode != 200 {
				t.Fatalf("request %d: expected 200, got %d", i+1, resp.StatusCode)
			}
		} else {
			if resp.StatusCode != 429 {
				t.Fatalf("request %d: expected 429, got %d", i+1, resp.StatusCode)
			}
			out := readEnvelope(t, resp.Body)
			if len(out.Errors) != 1 || out.Errors[0].Code != rtRateLimited.Code {
				t.Fatalf("unexpected envelope: %+v", out)
			}
		}
		resp.Body.Close()
	}
}

func TestAfterAuthWriteErrorStopsChain(t *testing.T) {
	tr, api := newAPI(routerHTTPConfig())
	var handlerRan atomic.Bool

	api.Routes("/", func(r *openapi.Router) {
		r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().Err(429, rtRateLimited),
			AfterAuth: []openapi.Middleware{
				func(_ fiber.Ctx, ctx openapi.RouteContext) error {
					return ctx.WriteError(rtRateLimited)
				},
			},
		}).Route("/limited").Get(openapi.Route{
			Handler: func(_ context.Context, _ struct{}) (ackOut, error) {
				handlerRan.Store(true)
				return ackOut{Response: openapi.Response{Message: "ok"}}, nil
			},
			Summary: "WriteError short-circuit",
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/limited", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	if handlerRan.Load() {
		t.Fatal("handler must not run after ctx.WriteError")
	}
}

func TestAfterAuthRunsAfterGuard(t *testing.T) {
	tr, _ := newAPI(routerHTTPConfig())
	api := openapi.New(tr, openapi.Config{
		Specs: []openapi.Spec{},
		Schemes: map[string]openapi.Scheme{
			"TestBearer": openapi.BearerScheme(openapi.BearerOptions{
				Verify: func(_ context.Context, cred openapi.Credential) (openapi.Identity, error) {
					if cred.Raw != "good-token" {
						return nil, rtUnauthorized
					}
					return "user-42", nil
				},
			}),
		},
	})

	var sawIdentity bool
	api.Routes("/", func(r *openapi.Router) {
		r.Group(openapi.GroupConfig{
			Security:  openapi.RequireSecurity("TestBearer"),
			Responses: openapi.Returns().Err(401, rtUnauthorized),
			AfterAuth: []openapi.Middleware{
				func(c fiber.Ctx, _ openapi.RouteContext) error {
					_, sawIdentity = openapi.IdentityFrom(c.Context())
					return nil
				},
			},
		}).Route("/me").Get(openapi.Route{
			Handler: func(_ context.Context, _ struct{}) (ackOut, error) {
				return ackOut{Response: openapi.Response{Message: "ok"}}, nil
			},
			Summary: "AfterAuth identity check",
		})
	})

	req := httptest.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	resp, err := tr.Fiber().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !sawIdentity {
		t.Fatal("AfterAuth middleware did not run after guard identity was set")
	}
}

func TestDeclaredHandlerErrorDoesNotHitGlobalHandler(t *testing.T) {
	tr, api := newAPI(routerHTTPConfig())
	api.Routes("/", func(r *openapi.Router) {
		r.Route("/boom").Get(openapi.Route{
			Handler: func(_ context.Context, _ struct{}) (ackOut, error) {
				return ackOut{}, rtInternal
			},
			Summary:   "Internal error declared on route",
			Responses: openapi.Returns().Err(500, rtInternal),
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/boom", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Fatalf("expected declared 500, got %d", resp.StatusCode)
	}
	out := readEnvelope(t, resp.Body)
	if len(out.Errors) != 1 || out.Errors[0].Code != rtInternal.Code {
		t.Fatalf("expected catalog error in envelope, got %+v", out)
	}
}
