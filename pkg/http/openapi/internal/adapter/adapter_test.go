package adapter_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/adapter"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/binder"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/compile"

	"github.com/gofiber/fiber/v3"
)

var adapterCatalog = errors.NewCatalog("adapter")

var errMissing = adapterCatalog.Define(errors.Def{Kind: errors.KindNotFound, Code: "10001", Message: "missing"})

type ackOut struct {
	openapi.Response
}

func okHandler(_ context.Context, _ struct{}) (ackOut, error) {
	return ackOut{Response: openapi.Response{Message: "ok"}}, nil
}

func missingHandler(_ context.Context, _ struct{}) (ackOut, error) {
	return ackOut{}, errMissing
}

func compileHandler(handler any, errCases []compile.ErrCase, enveloped bool) fiber.Handler {
	inType, outType, call := mustParse(handler)
	route := &compile.Route{
		Call:        call,
		Enveloped:   enveloped,
		SuccessType: outType,
		InType:      inType,
		Binder:      binder.New(inType, nil),
		ErrCases:    errCases,
	}
	return adapter.Handler(route, adapterCatalog)
}

func mustParse(handler any) (inType, outType reflect.Type, call func(context.Context, any) (any, error)) {
	t := reflect.TypeOf(handler)
	inType = t.In(1)
	if inType.Kind() == reflect.Pointer {
		inType = inType.Elem()
	}
	outType = t.Out(0)
	if outType.Kind() == reflect.Pointer {
		outType = outType.Elem()
	}
	hv := reflect.ValueOf(handler)
	call = func(ctx context.Context, in any) (any, error) {
		var inVal reflect.Value
		if in == nil {
			inVal = reflect.Zero(t.In(1))
		} else {
			inVal = reflect.ValueOf(in)
		}
		out := hv.Call([]reflect.Value{reflect.ValueOf(ctx), inVal})
		if !out[1].IsNil() {
			return nil, out[1].Interface().(error)
		}
		return out[0].Interface(), nil
	}
	return inType, outType, call
}

func TestAdapterEnvelopedSuccess(t *testing.T) {
	app := fiber.New()
	app.Get("/ok", compileHandler(okHandler, nil, true))

	resp, err := app.Test(httptest.NewRequest("GET", "/ok", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "success" || out.Message != "ok" {
		t.Fatalf("unexpected body: %+v", out)
	}
}

func TestAdapterDeclaredError(t *testing.T) {
	app := fiber.New()
	app.Get("/missing", compileHandler(missingHandler, []compile.ErrCase{
		{Status: 404, Errors: []*errors.Error{errMissing}},
	}, true))

	resp, err := app.Test(httptest.NewRequest("GET", "/missing", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Status string `json:"status"`
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "error" || len(out.Errors) != 1 || out.Errors[0].Code != "10001" {
		t.Fatalf("unexpected error body: %+v", out)
	}
}
