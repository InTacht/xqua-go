package compile_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/compile"
)

var testCatalog = errors.NewCatalog("compile-test")

var (
	errNotFound   = testCatalog.Define(errors.Def{Kind: errors.KindNotFound, Code: "10001", Message: "not found"})
	errConflict   = testCatalog.Define(errors.Def{Kind: errors.KindConflict, Code: "10002", Message: "conflict"})
	errValidation = testCatalog.Define(errors.Def{Kind: errors.KindValidation, Code: "10003", Message: "validation failed"})
)

func errCases422(extra ...*errors.Error) []compile.ErrCase {
	return []compile.ErrCase{{Status: 422, Errors: append([]*errors.Error{errValidation}, extra...)}}
}

type in struct {
	ID int64 `path:"id"`
}

type out struct {
	openapi.Response
	Name string `json:"name"`
}

func handler(_ context.Context, _ in) (out, error) { return out{}, nil }

func TestBuildTypedRoute(t *testing.T) {
	route := compile.Build(compile.Input{
		Method:       "GET",
		Path:         "/users/:id",
		Handler:      handler,
		InferSuccess: true,
		ErrCases:     errCases422(),
		Catalog:      testCatalog,
		KindStatuses: openapi.DefaultKindStatuses(),
	})
	if route.Binder == nil || route.Call == nil {
		t.Fatal("expected binder and call")
	}
	if route.InType != reflect.TypeFor[in]() {
		t.Fatalf("unexpected in type %v", route.InType)
	}
	if route.SuccessType != reflect.TypeFor[out]() {
		t.Fatalf("unexpected out type %v", route.SuccessType)
	}
}

func TestBuildDescribeRequiresSuccessOrExtra(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	compile.Build(compile.Input{
		Method:   "GET",
		Path:     "/ws",
		Describe: true,
		Catalog:  testCatalog,
	})
}

func TestBuildDescribeAllowsExtraOnly(t *testing.T) {
	route := compile.Build(compile.Input{
		Method:   "GET",
		Path:     "/ws",
		Describe: true,
		Extra:    []compile.ResponseDecl{{Status: 101}},
		Catalog:  testCatalog,
	})
	if route.Call != nil {
		t.Fatal("describe route must not call handler")
	}
}

func TestBuildRejectsMismatchedReturns(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	compile.Build(compile.Input{
		Method:       "GET",
		Path:         "/users/:id",
		Handler:      handler,
		SuccessType:  reflect.TypeFor[struct{}](),
		ErrCases:     errCases422(),
		Catalog:      testCatalog,
		KindStatuses: openapi.DefaultKindStatuses(),
	})
}

func TestBuildRequires422ForBindableInput(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	compile.Build(compile.Input{
		Method:       "GET",
		Path:         "/users/:id",
		Handler:      handler,
		InferSuccess: true,
		Catalog:      testCatalog,
		KindStatuses: openapi.DefaultKindStatuses(),
	})
}

func TestBuildValidatesErrStatus(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	compile.Build(compile.Input{
		Method:       "GET",
		Path:         "/users/:id",
		Handler:      handler,
		ErrCases:     append(errCases422(), compile.ErrCase{Status: 404, Errors: []*errors.Error{errConflict}}),
		Catalog:      testCatalog,
		KindStatuses: openapi.DefaultKindStatuses(),
	})
}

func TestBuildAcceptsDeclaredErr(t *testing.T) {
	route := compile.Build(compile.Input{
		Method:  "GET",
		Path:    "/users/:id",
		Handler: handler,
		ErrCases: append(errCases422(),
			compile.ErrCase{Status: 404, Errors: []*errors.Error{errNotFound}}),
		Catalog:      testCatalog,
		KindStatuses: openapi.DefaultKindStatuses(),
	})
	if route.ErrIndex[errNotFound] != 404 {
		t.Fatalf("unexpected err index: %+v", route.ErrIndex)
	}
}
