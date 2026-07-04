// Run: go run ./examples/hello
//
// Minimal xqua-go process: a logger, one HTTP unit, and runtime.Run.
// GET http://localhost:8080/ returns a standard RES envelope.
package main

import (
	"log"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

// Ctx is empty here; real shared deps live in examples/api.
type Ctx struct{}

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
		return http.New(http.Config{
			Logger:  log.Derive("http"),
			Catalog: catalog,
		}).Routes("/", func(r *http.Router) {
			r.Get("/", func(c fiber.Ctx) error {
				return http.RES(c).
					Message("hello").
					Data("service", "hello").
					Ok()
			})
		})
	})

	return r.Run()
}
