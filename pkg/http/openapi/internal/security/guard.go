package security

import (
	"context"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/adapter"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/compile"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/wire"

	"github.com/gofiber/fiber/v3"
)

// Requirement is one OpenAPI security alternative (OR branch).
type Requirement struct {
	Names  []string
	Scopes []string
}

// Scheme is the runtime half of a registered security scheme.
type Scheme struct {
	Verify  func(ctx context.Context, cred Credential) (any, error)
	Extract func(c fiber.Ctx) (raw string, ok bool)
}

// Credential is passed to Verify during authentication.
type Credential struct {
	Scheme string
	Raw    string
	Scopes []string
}

// Guard wraps a Fiber handler with scheme enforcement.
func Guard(
	route *compile.Route,
	requirements []Requirement,
	schemes map[string]Scheme,
	catalog *errors.Catalog,
	unauthorized *errors.Error,
	next fiber.Handler,
) fiber.Handler {
	if len(requirements) == 0 {
		return next
	}
	return func(c fiber.Ctx) error {
		var forbiddenErr error
		for _, req := range requirements {
			if len(req.Names) == 0 {
				continue
			}
			name := req.Names[0]
			scheme, ok := schemes[name]
			if !ok || scheme.Verify == nil || scheme.Extract == nil {
				continue
			}
			raw, ok := scheme.Extract(c)
			if !ok {
				continue
			}
			id, err := scheme.Verify(c.Context(), Credential{
				Scheme: name,
				Raw:    raw,
				Scopes: req.Scopes,
			})
			if err != nil {
				if st, ok := ResolveAuthStatus(err, route.ErrCases); ok && st == 403 {
					forbiddenErr = err
				}
				continue
			}
			c.SetContext(wire.WithIdentity(c.Context(), id))
			return next(c)
		}
		if forbiddenErr != nil {
			return adapter.WriteHandlerError(c, forbiddenErr, route, catalog)
		}
		if unauthorized != nil {
			return adapter.WriteHandlerError(c, unauthorized, route, catalog)
		}
		return adapter.WriteHandlerError(c, errors.NewPlain("unauthorized"), route, catalog)
	}
}
