package openapi_test

import (
	"context"
	"testing"

	"github.com/InTacht/xqua-go/pkg/http/openapi"
)

type demoUser struct {
	ID   int64
	Name string
}

func TestIdentityContext(t *testing.T) {
	ctx := context.Background()
	user := demoUser{ID: 1, Name: "ada"}

	ctx = openapi.WithIdentity(ctx, user)

	got, ok := openapi.IdentityFrom(ctx)
	if !ok {
		t.Fatal("expected identity in context")
	}
	if got.(demoUser).Name != "ada" {
		t.Fatalf("unexpected identity: %+v", got)
	}

	typed, ok := openapi.IdentityAs[demoUser](ctx)
	if !ok || typed.ID != 1 {
		t.Fatalf("IdentityAs failed: %+v ok=%v", typed, ok)
	}

	_, ok = openapi.IdentityAs[string](ctx)
	if ok {
		t.Fatal("expected IdentityAs type mismatch")
	}
}

func TestIdentityFromEmpty(t *testing.T) {
	if _, ok := openapi.IdentityFrom(context.Background()); ok {
		t.Fatal("expected no identity")
	}
}
