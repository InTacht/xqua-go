package openapi

import (
	"errors"

	xerrors "github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/adapter"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/compile"

	"github.com/gofiber/fiber/v3"
)

// responseWritten stops AfterAuth when ctx.WriteError successfully wrote a response.
type responseWritten struct{ catalog error }

func (e responseWritten) Error() string { return "openapi: response written" }

func (e responseWritten) Unwrap() error { return e.catalog }

// RouteContext carries per-request helpers for openapi group middleware.
type RouteContext struct {
	// WriteError writes a declared catalog error through the route envelope.
	WriteError func(error) error
}

// Middleware runs after security guard and before request binding.
// Return nil to continue, or an error to short-circuit. Use ctx.WriteError
// for catalog errors declared on the route or group Responses.Err(...).
type Middleware func(c fiber.Ctx, ctx RouteContext) error

func wrapAfterAuth(
	route *compile.Route,
	catalog *xerrors.Catalog,
	middlewares []Middleware,
	next fiber.Handler,
) fiber.Handler {
	if len(middlewares) == 0 {
		return next
	}
	return func(c fiber.Ctx) error {
		ctx := RouteContext{
			WriteError: func(err error) error {
				if werr := adapter.WriteHandlerError(c, err, route, catalog); werr != nil {
					return werr
				}
				return responseWritten{catalog: err}
			},
		}
		for _, mw := range middlewares {
			if err := mw(c, ctx); err != nil {
				var written responseWritten
				if errors.As(err, &written) {
					return nil
				}
				return adapter.WriteHandlerError(c, err, route, catalog)
			}
		}
		return next(c)
	}
}
