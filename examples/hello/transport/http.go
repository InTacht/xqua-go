package transport

import (
	"github.com/InTacht/xqua-go/examples/hello/ctx"

	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/transport"
	"github.com/InTacht/xqua-go/pkg/transport/http"

	"github.com/gofiber/fiber/v3"
)

func HTTP(c *ctx.Ctx, log *logger.Logger) transport.Transport {
	return http.New(http.Config{
		Host:        "0.0.0.0",
		Port:        8080,
		Logger:      log,
		FiberConfig: fiber.Config{ServerHeader: "hello"},
	}).Routes("/", func(r fiber.Router) {
		r.Get("/", func(c fiber.Ctx) error {
			return c.JSON(fiber.Map{
				"message": "hello from xqua-go",
				"service": "hello",
			})
		})
	})
}
