package transport

import (
	"context"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

var (
	publicCatalog = errors.NewCatalog("public")
	adminCatalog  = errors.NewCatalog("admin")
)

type rootIn struct{}

type rootOut struct {
	openapi.Response
	Data struct {
		Port int `json:"port"`
	} `json:"data"`
}

func Public(name string, log runtime.Logger) runtime.Unit {
	t := http.New(http.Config{
		Port:        8080,
		Logger:      log,
		Catalog:     publicCatalog,
		FiberConfig: fiber.Config{ServerHeader: name + "-public"},
	})
	openapi.New(t, openapi.Config{
		Specs: []openapi.Spec{{Path: "/openapi.json", Title: name + " Public API"}},
	}).Routes("/", func(r *openapi.Router) {
		r.Route("/").Get(openapi.Route{
			Handler:   publicRoot,
			Summary:   "Public root",
			Responses: openapi.Returns(),
		})
	})
	return t
}

func publicRoot(_ context.Context, _ rootIn) (rootOut, error) {
	var out rootOut
	out.Message = "public api"
	out.Data.Port = 8080
	return out, nil
}

func Admin(name string, log runtime.Logger) runtime.Unit {
	t := http.New(http.Config{
		Port:        8081,
		Logger:      log,
		Catalog:     adminCatalog,
		FiberConfig: fiber.Config{ServerHeader: name + "-admin"},
	})
	openapi.New(t, openapi.Config{
		Specs: []openapi.Spec{{Path: "/openapi.json", Title: name + " Admin API"}},
	}).Routes("/", func(r *openapi.Router) {
		r.Route("/").Get(openapi.Route{
			Handler:   adminRoot,
			Summary:   "Admin root",
			Responses: openapi.Returns(),
		})
	})
	return t
}

func adminRoot(_ context.Context, _ rootIn) (rootOut, error) {
	var out rootOut
	out.Message = "admin api"
	out.Data.Port = 8081
	return out, nil
}
