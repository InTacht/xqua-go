package openapi

import (
	"context"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/compile"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/security"

	"github.com/gofiber/fiber/v3"
)

func toSecurityRequirements(reqs []SecurityRequirement) []security.Requirement {
	if len(reqs) == 0 {
		return nil
	}
	out := make([]security.Requirement, len(reqs))
	for i, req := range reqs {
		for name, scopes := range req {
			out[i] = security.Requirement{
				Names:  []string{name},
				Scopes: append([]string(nil), scopes...),
			}
		}
	}
	return out
}

func toSecuritySchemes(schemes map[string]Scheme) map[string]security.Scheme {
	if len(schemes) == 0 {
		return nil
	}
	out := make(map[string]security.Scheme, len(schemes))
	for name, scheme := range schemes {
		s := scheme
		out[name] = security.Scheme{
			Verify: func(ctx context.Context, cred security.Credential) (any, error) {
				if s.Verify == nil {
					return nil, nil
				}
				return s.Verify(ctx, Credential{
					Scheme: cred.Scheme,
					Raw:    cred.Raw,
					Scopes: cred.Scopes,
				})
			},
			Extract: func(c fiber.Ctx) (string, bool) {
				if s.Extract == nil {
					return "", false
				}
				return s.Extract(c)
			},
		}
	}
	return out
}

func wrapWithSecurity(
	route *compile.Route,
	requirements []SecurityRequirement,
	schemes map[string]Scheme,
	catalog *errors.Catalog,
	next fiber.Handler,
) fiber.Handler {
	if len(requirements) == 0 {
		return next
	}
	ValidateRequirements("openapi", requirements)
	return security.Guard(
		route,
		toSecurityRequirements(requirements),
		toSecuritySchemes(schemes),
		catalog,
		route.Unauthorized,
		next,
	)
}

func validateSecuredRoute(prefix string, requirements []SecurityRequirement, schemes map[string]Scheme, errCases []compile.ErrCase, describe bool) {
	if describe || len(requirements) == 0 {
		return
	}
	ValidateRequirements(prefix, requirements)
	for i, req := range requirements {
		for name := range req {
			scheme, ok := schemes[name]
			if !ok {
				panic(prefix + ": security requirement references unknown scheme " + name)
			}
			if scheme.Verify == nil {
				panic(prefix + ": scheme " + name + " has no Verify func (required for enforced routes)")
			}
			_ = i
		}
	}
	security.ValidateRoute(prefix, toSecurityRequirements(requirements), errCases, describe)
}
