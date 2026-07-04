package transport

import (
	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

var (
	publicCatalog = errors.NewCatalog("public")
	adminCatalog  = errors.NewCatalog("admin")
)

// Public is the customer-facing HTTP unit on :8080.
//
//	curl http://127.0.0.1:8080/
func Public(name string, log runtime.Logger) runtime.Unit {
	return http.New(http.Config{
		Port:        8080,
		Logger:      log,
		Catalog:     publicCatalog,
		FiberConfig: fiber.Config{ServerHeader: name + "-public"},
	}).Routes("/", func(r *http.Router) {
		r.Get("/", func(c fiber.Ctx) error {
			return http.RES(c).
				Message("public api").
				Data("port", 8080).
				Ok()
		})
	})
}

// Admin is a separate HTTP unit on :8081 (different trust boundary / port).
//
//	curl http://127.0.0.1:8081/
func Admin(name string, log runtime.Logger) runtime.Unit {
	return http.New(http.Config{
		Port:        8081,
		Logger:      log,
		Catalog:     adminCatalog,
		FiberConfig: fiber.Config{ServerHeader: name + "-admin"},
	}).Routes("/", func(r *http.Router) {
		r.Get("/", func(c fiber.Ctx) error {
			return http.RES(c).
				Message("admin api").
				Data("port", 8081).
				Ok()
		})
	})
}
