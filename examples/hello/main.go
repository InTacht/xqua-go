// Run: go run ./examples/hello
//
// Smallest xqua-go HTTP process: runtime, one typed route, RES envelope,
// and a generated OpenAPI document at /openapi.json.
//
// Next: examples/showcase for the full HTTP/OpenAPI surface.
package main

import (
	"context"
	"log"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/runtime"
)

type Ctx struct{}

type helloIn struct{}

type helloOut struct {
	openapi.Response
	Data struct {
		Service string `json:"service"`
	} `json:"data"`
}

func hello(_ context.Context, _ helloIn) (helloOut, error) {
	var out helloOut
	out.Message = "hello"
	out.Data.Service = "hello"
	return out, nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	appLog := logger.New(&logger.Config{
		Name: "hello",
		ID:   "hello",
	})

	defer appLog.Close()

	r, err := runtime.New(&Ctx{}, appLog)
	if err != nil {
		return err
	}

	catalog := errors.NewCatalog("hello")
	r.Unit(func(_ *Ctx, log runtime.Logger) runtime.Unit {
		t := http.New(http.Config{
			Logger:  log.Derive("http"),
			Catalog: catalog,
			Version: "1.0.0",
		})
		openapi.New(t, openapi.Config{
			Specs: []openapi.Spec{{Path: "/openapi.json", Title: "Hello API", Version: "1.0.0"}},
		}).Routes("/", func(r *openapi.Router) {
			r.Route("/").Get(openapi.Route{
				Handler:   hello,
				Summary:   "Say hello",
				Responses: openapi.Returns(),
			})
		})
		return t
	})

	return r.Run()
}
