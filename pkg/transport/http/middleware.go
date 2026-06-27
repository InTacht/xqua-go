package http

import (
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/gofiber/fiber/v3"
)

// RequestContext bridges Fiber request IDs into the standard logger context.
func RequestContext() fiber.Handler {
	return func(c fiber.Ctx) error {
		if id, ok := c.Locals("requestid").(string); ok && id != "" {
			ctx := logger.ContextWithRequestID(c.Context(), id)
			c.SetContext(ctx)
		}
		return c.Next()
	}
}
