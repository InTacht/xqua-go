package http_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/gofiber/fiber/v3"
)

var resTestCatalog = errors.NewCatalog("response")

var (
	errValidation = resTestCatalog.Define(errors.Def{
		Code: "422301", Message: "external_id is required", Source: "body.external_id",
	})
	errInternal = resTestCatalog.Define(errors.Def{
		Code: "500001", Message: "database unavailable",
	})
	errInternalOuter = resTestCatalog.Define(errors.Def{
		Code: "500002", Message: "service failed",
	})
	errInternalMiddle = resTestCatalog.Define(errors.Def{
		Code: "500003", Message: "repository failed",
	})
	errMultiValidation = resTestCatalog.Define(errors.Def{
		Code: "422302", Message: "second validation error",
	})
	errSoftFailure = resTestCatalog.Define(errors.Def{
		Code: "200305", Message: "soft failure reported in body",
	})
)

func TestSuccessEnvelope(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return http.RES(c).
			Message("Subscriber upserted").
			Data("subscriber", map[string]any{"id": "sub_1"}).
			Ok()
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}

	if out["status"] != "success" {
		t.Fatalf("expected success status, got %v", out["status"])
	}
	if _, ok := out["errors"]; ok {
		t.Fatal("errors should be omitted on success")
	}
}

func TestErrorEnvelope(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return http.RES(c).
			Message("validation failed").
			Error(errValidation).
			Status(422).
			Ok()
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Status string             `json:"status"`
		Errors []http.ErrorDetail `json:"errors"`
		Data   any                `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}

	if out.Status != "error" {
		t.Fatalf("expected error status, got %s", out.Status)
	}
	if out.Data != nil {
		t.Fatal("data should be omitted on error")
	}
	if len(out.Errors) != 1 || out.Errors[0].Code != "422301" {
		t.Fatalf("unexpected errors: %+v", out.Errors)
	}
	if out.Errors[0].Cause != "" {
		t.Fatal("cause should be omitted when error is not wrapped")
	}
}

func TestErrorEnvelopeWithCause(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		cause := errors.NewPlain("connection reset")
		err := errors.Wrap(cause, errInternal)
		return http.RES(c).Error(errors.AsErrors(err)[0]).Status(500).Ok()
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Errors []http.ErrorDetail `json:"errors"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}

	if len(out.Errors) != 1 {
		t.Fatalf("expected 1 error, got %+v", out.Errors)
	}
	if out.Errors[0].Cause != "connection reset" {
		t.Fatalf("expected cause on wrapped error, got %#v", out.Errors[0])
	}
}

func TestErrorEnvelopeImmediateCauseOnly(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		inner := errors.NewPlain("db timeout")
		middle := errors.Wrap(inner, errInternalMiddle)
		outer := errors.Wrap(middle, errInternalOuter)
		return http.RES(c).Error(errors.AsErrors(outer)[0]).Status(500).Ok()
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Errors []http.ErrorDetail `json:"errors"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}

	if out.Errors[0].Code != "500002" {
		t.Fatalf("expected outer error, got %+v", out.Errors[0])
	}
	if out.Errors[0].Cause != "response<500003>: repository failed" {
		t.Fatalf("expected immediate cause only, got %#v", out.Errors[0].Cause)
	}
}

func TestApplyErrorsCollection(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		errs := errors.Errors{errValidation, errMultiValidation}
		return http.RES(c).Message("validation failed").ApplyErrors(errs).Status(422).Ok()
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestErrorEnvelopeDefaultsToHTTP200(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return http.RES(c).Message("soft failure").Error(errSoftFailure).Ok()
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for status-less error envelope, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Status string             `json:"status"`
		Errors []http.ErrorDetail `json:"errors"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "error" || len(out.Errors) != 1 || out.Errors[0].Code != "200305" {
		t.Fatalf("unexpected envelope: %+v", out)
	}
}
