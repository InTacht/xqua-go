package http

import (
	"github.com/InTacht/xqua-go/pkg/errors"

	"github.com/gofiber/fiber/v3"
)

// KindStatuses maps semantic error kinds (errors.Kind*) to HTTP status codes.
// It is the conventional default applied by the Router when a returned catalog
// error has no explicit per-route Status/Statuses mapping. Unknown kinds fall
// back to Config.DefaultStatus.
type KindStatuses map[string]int

// DefaultKindStatuses returns the canonical kind→status table. Callers may copy
// and adjust it, then set it on Config.KindStatuses to override the defaults.
func DefaultKindStatuses() KindStatuses {
	return KindStatuses{
		errors.KindValidation:   fiber.StatusUnprocessableEntity, // 422
		errors.KindNotFound:     fiber.StatusNotFound,            // 404
		errors.KindConflict:     fiber.StatusConflict,            // 409
		errors.KindUnauthorized: fiber.StatusUnauthorized,        // 401
		errors.KindForbidden:    fiber.StatusForbidden,           // 403
		errors.KindRateLimit:    fiber.StatusTooManyRequests,     // 429
		errors.KindInternal:     fiber.StatusInternalServerError, // 500
	}
}

// StandardErrors defines conventional Unhandled and NotFound fallback entries
// on catalog and returns them as Fallbacks, so services stop hand-writing the
// two required fallbacks. Unhandled uses kind internal; NotFound uses kind
// not_found. Call once during catalog setup:
//
//	var API = errors.NewCatalog("api")
//	var fallbacks = http.StandardErrors(API)
func StandardErrors(catalog *errors.Catalog) Fallbacks {
	return Fallbacks{
		Unhandled: catalog.Define(errors.Def{
			Kind: errors.KindInternal, Code: "internal", Message: "internal error",
		}),
		NotFound: catalog.Define(errors.Def{
			Kind: errors.KindNotFound, Code: "not_found", Message: "route not found",
		}),
	}
}
