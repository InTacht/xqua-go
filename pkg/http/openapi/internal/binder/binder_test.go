package binder_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/binder"

	"github.com/gofiber/fiber/v3"
)

var binderCatalog = errors.NewCatalog("binder")

var (
	errValidation = binderCatalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "10001", Message: "validation failed",
	})
	errIDInvalid = binderCatalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "10002", Message: "id is invalid", Source: "params.id",
	})
)

type jsonIn struct {
	Email string `json:"email"`
}

type jsonRequiredIn struct {
	Username string `json:"username" required:"true"`
	Password string `json:"password" required:"true"`
}

func TestBindJSONBody(t *testing.T) {
	app := fiber.New()
	app.Post("/users", func(c fiber.Ctx) error {
		b := binder.New(reflect.TypeFor[jsonIn](), nil)
		in, err := b.BindRequest(c)
		if err != nil {
			return c.Status(422).SendString(err.Error())
		}
		got := in.(jsonIn)
		if got.Email != "a@b.c" {
			t.Fatalf("unexpected bind: %+v", got)
		}
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("POST", "/users", strings.NewReader(`{"email":"a@b.c"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestBindJSONRequiredFields(t *testing.T) {
	app := fiber.New()
	app.Post("/login", func(c fiber.Ctx) error {
		b := binder.New(reflect.TypeFor[jsonRequiredIn](), []*errors.Error{errValidation})
		_, err := b.BindRequest(c)
		if err == nil {
			return c.SendStatus(200)
		}
		es, ok := err.(errors.Errors)
		if !ok || len(es) != 2 {
			t.Fatalf("expected two validation errors, got %v", err)
		}
		return c.SendStatus(422)
	})

	req := httptest.NewRequest("POST", "/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestBindQueryDefaultSkipsMinimumOnMissing(t *testing.T) {
	type auditIn struct {
		Limit int `query:"limit" default:"20" minimum:"1" maximum:"100"`
	}
	app := fiber.New()
	app.Get("/audit", func(c fiber.Ctx) error {
		b := binder.New(reflect.TypeFor[auditIn](), []*errors.Error{errValidation})
		in, err := b.BindRequest(c)
		if err != nil {
			return c.Status(422).SendString(err.Error())
		}
		if got := in.(auditIn).Limit; got != 20 {
			t.Fatalf("expected default limit 20, got %d", got)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/audit", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

type pathIn struct {
	ID int64 `path:"id"`
}

func TestBindInvalidPathParamUsesCatalogCode(t *testing.T) {
	app := fiber.New()
	app.Get("/items/:id", func(c fiber.Ctx) error {
		b := binder.New(reflect.TypeFor[pathIn](), []*errors.Error{errIDInvalid, errValidation})
		_, err := b.BindRequest(c)
		if err == nil {
			return c.SendStatus(200)
		}
		es, ok := err.(errors.Errors)
		if !ok || len(es) != 1 {
			t.Fatalf("expected validation errors, got %v", err)
		}
		if es[0].Code != errIDInvalid.Code {
			t.Fatalf("expected catalog code %q, got %q", errIDInvalid.Code, es[0].Code)
		}
		if es[0].Source != "params.id" {
			t.Fatalf("expected source params.id, got %q", es[0].Source)
		}
		return c.SendStatus(422)
	})

	req := httptest.NewRequest("GET", "/items/nope", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestBindInvalidPathParamFallsBackToGenericValidation(t *testing.T) {
	app := fiber.New()
	app.Get("/items/:id", func(c fiber.Ctx) error {
		b := binder.New(reflect.TypeFor[pathIn](), []*errors.Error{errValidation})
		_, err := b.BindRequest(c)
		if err == nil {
			return c.SendStatus(200)
		}
		es := err.(errors.Errors)
		if es[0].Code != errValidation.Code {
			t.Fatalf("expected generic validation code %q, got %q", errValidation.Code, es[0].Code)
		}
		return c.SendStatus(422)
	})

	req := httptest.NewRequest("GET", "/items/nope", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

type uploadIn struct {
	Title string                `form:"title"`
	File  *multipart.FileHeader `form:"file" required:"true"`
}

func TestBindMultipartUpload(t *testing.T) {
	app := fiber.New()
	app.Post("/upload", func(c fiber.Ctx) error {
		b := binder.New(reflect.TypeFor[uploadIn](), []*errors.Error{errValidation})
		in, err := b.BindRequest(c)
		if err != nil {
			return c.Status(422).SendString(err.Error())
		}
		got := in.(uploadIn)
		if got.Title != "report" {
			t.Fatalf("expected title report, got %q", got.Title)
		}
		if got.File == nil || got.File.Filename != "doc.pdf" {
			t.Fatalf("unexpected file: %+v", got.File)
		}
		f, err := got.File.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		body, _ := io.ReadAll(f)
		if string(body) != "pdf-bytes" {
			t.Fatalf("unexpected file body %q", body)
		}
		return c.SendStatus(200)
	})

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.WriteField("title", "report")
	part, _ := w.CreateFormFile("file", "doc.pdf")
	_, _ = part.Write([]byte("pdf-bytes"))
	_ = w.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

type multiUploadIn struct {
	Files []*multipart.FileHeader `form:"files" required:"true"`
}

func TestBindMultipartMultipleFiles(t *testing.T) {
	app := fiber.New()
	app.Post("/upload", func(c fiber.Ctx) error {
		b := binder.New(reflect.TypeFor[multiUploadIn](), []*errors.Error{errValidation})
		in, err := b.BindRequest(c)
		if err != nil {
			return c.Status(422).SendString(err.Error())
		}
		got := in.(multiUploadIn)
		if len(got.Files) != 2 {
			t.Fatalf("expected 2 files, got %d", len(got.Files))
		}
		return c.SendStatus(200)
	})

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for _, name := range []string{"a.txt", "b.txt"} {
		part, _ := w.CreateFormFile("files", name)
		_, _ = part.Write([]byte("x"))
	}
	_ = w.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

type queryRequiredIn struct {
	Filter string `query:"filter" required:"true"`
}

func TestBindQueryRequiredField(t *testing.T) {
	app := fiber.New()
	app.Get("/search", func(c fiber.Ctx) error {
		b := binder.New(reflect.TypeFor[queryRequiredIn](), []*errors.Error{errValidation})
		_, err := b.BindRequest(c)
		if err == nil {
			return c.SendStatus(200)
		}
		es, ok := err.(errors.Errors)
		if !ok || len(es) != 1 || es[0].Source != "query.filter" {
			t.Fatalf("expected query.filter required error, got %v", err)
		}
		return c.SendStatus(422)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/search", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

type headerRequiredIn struct {
	Token string `header:"X-Token" required:"true"`
}

func TestBindHeaderRequiredField(t *testing.T) {
	app := fiber.New()
	app.Get("/secure", func(c fiber.Ctx) error {
		b := binder.New(reflect.TypeFor[headerRequiredIn](), []*errors.Error{errValidation})
		_, err := b.BindRequest(c)
		if err == nil {
			return c.SendStatus(200)
		}
		es, ok := err.(errors.Errors)
		if !ok || len(es) != 1 || es[0].Source != "header.X-Token" {
			t.Fatalf("expected header.X-Token required error, got %v", err)
		}
		return c.SendStatus(422)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/secure", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

type cookieRequiredIn struct {
	Session string `cookie:"session" required:"true"`
}

func TestBindCookieRequiredField(t *testing.T) {
	app := fiber.New()
	app.Get("/session", func(c fiber.Ctx) error {
		b := binder.New(reflect.TypeFor[cookieRequiredIn](), []*errors.Error{errValidation})
		_, err := b.BindRequest(c)
		if err == nil {
			return c.SendStatus(200)
		}
		es, ok := err.(errors.Errors)
		if !ok || len(es) != 1 || es[0].Source != "cookie.session" {
			t.Fatalf("expected cookie.session required error, got %v", err)
		}
		return c.SendStatus(422)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/session", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestBindMultipartMissingRequiredFile(t *testing.T) {
	app := fiber.New()
	app.Post("/upload", func(c fiber.Ctx) error {
		b := binder.New(reflect.TypeFor[uploadIn](), []*errors.Error{errValidation})
		_, err := b.BindRequest(c)
		if err == nil {
			return c.SendStatus(200)
		}
		es, ok := err.(errors.Errors)
		if !ok || len(es) != 1 || es[0].Source != "form.file" {
			t.Fatalf("expected form.file required error, got %v", err)
		}
		return c.SendStatus(422)
	})

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.WriteField("title", "report")
	_ = w.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}
