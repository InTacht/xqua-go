package openapi_test

import (
	"testing"

	"github.com/InTacht/xqua-go/pkg/http/openapi"
)

func TestResolveSecurityInheritPublicRequire(t *testing.T) {
	defaultSpec := openapi.RequireSecurity("Default")
	group := openapi.RequireSecurity("Group")
	route := openapi.PublicSecurity()

	reqs, public := openapi.ResolveSecurity(route, []openapi.SecuritySpec{group}, defaultSpec)
	if !public || len(reqs) != 0 {
		t.Fatalf("expected public route, got reqs=%v public=%v", reqs, public)
	}

	reqs, public = openapi.ResolveSecurity(openapi.InheritSecurity(), []openapi.SecuritySpec{group}, defaultSpec)
	if public || len(reqs) != 1 {
		t.Fatalf("expected group security, got %+v public=%v", reqs, public)
	}
	if _, ok := reqs[0]["Group"]; !ok {
		t.Fatalf("expected Group scheme, got %+v", reqs)
	}

	reqs, public = openapi.ResolveSecurity(openapi.InheritSecurity(), nil, defaultSpec)
	if public || len(reqs) != 1 {
		t.Fatalf("expected default security, got %+v", reqs)
	}
	if _, ok := reqs[0]["Default"]; !ok {
		t.Fatalf("expected Default scheme, got %+v", reqs)
	}
}

func TestRequireAnySecurity(t *testing.T) {
	spec := openapi.RequireAnySecurity("Bearer", "ApiKey")
	reqs, public := openapi.ResolveSecurity(spec, nil, openapi.InheritSecurity())
	if public || len(reqs) != 2 {
		t.Fatalf("expected two OR alternatives, got %+v", reqs)
	}
}

func TestMergeSecuritySchemes(t *testing.T) {
	merged := openapi.MergeSecuritySchemes(
		map[string]*openapi.SecurityScheme{"DocOnly": {Type: "apiKey", Name: "X", In: openapi.InHeader}},
		map[string]openapi.Scheme{
			"Bearer": openapi.BearerScheme(openapi.BearerOptions{}),
		},
	)
	if merged["DocOnly"] == nil || merged["Bearer"] == nil {
		t.Fatalf("expected merged schemes, got %+v", merged)
	}
	if merged["Bearer"].Type != "http" || merged["Bearer"].Scheme != "bearer" {
		t.Fatalf("unexpected bearer scheme: %+v", merged["Bearer"])
	}
}
