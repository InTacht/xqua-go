package openapi

import (
	"github.com/InTacht/xqua-go/pkg/errors"

	"github.com/gofiber/fiber/v3"
)

// KindStatuses maps semantic error kinds (errors.Kind*) to HTTP status codes.
// It is the conventional default applied by the engine's Router when a returned
// catalog error has no explicit per-route Status/Statuses mapping. Unknown
// kinds fall back to the transport's DefaultStatus.
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
