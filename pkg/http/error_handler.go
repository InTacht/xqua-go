package http

import (
	stderrors "errors"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

// ErrorHandler converts errors that reach Fiber into the RES envelope. It backs
// three cases: Fiber errors (e.g. unmatched routes) use the Fiber status;
// public-catalog errors that no route mapped use defaultStatus; everything
// else — plain errors and internal catalog errors that must not leak — is
// replaced by the Unhandled fallback with defaultStatus.
func ErrorHandler(log runtime.Logger, catalog *errors.Catalog, fallbacks Fallbacks, defaultStatus int) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		if err == nil {
			return nil
		}

		var fe *fiber.Error
		if stderrors.As(err, &fe) {
			entry := fallbacks.Unhandled
			message := "request failed"
			if fe.Code == fiber.StatusNotFound {
				entry = fallbacks.NotFound
				message = "not found"
			}
			return RES(c).Message(message).Error(entry).Status(fe.Code).Ok()
		}

		if inCatalog(catalog, err) {
			log.ErrorCtx(c.Context(), err, "request failed")
			return RES(c).Message("request failed").Apply(err).Status(defaultStatus).Ok()
		}

		if errors.IsStructured(err) {
			// Internal catalog errors are implementation details: log the real
			// chain, render only the public fallback.
			log.ErrorCtx(c.Context(), errors.Wrap(err, fallbacks.Unhandled), "internal error not mapped to public catalog")
			return RES(c).Message("internal error").Error(fallbacks.Unhandled).Status(defaultStatus).Ok()
		}

		mapped := errors.MapOr(err, fallbacks.Unhandled)
		log.ErrorCtx(c.Context(), mapped, "request failed")
		return RES(c).Message("internal error").Apply(mapped).Status(defaultStatus).Ok()
	}
}

// inCatalog reports whether err carries canonical errors and every top-level
// entry belongs to the public catalog.
func inCatalog(catalog *errors.Catalog, err error) bool {
	entries := errors.AsErrors(err)
	if len(entries) == 0 {
		return false
	}
	for _, entry := range entries {
		if !catalog.Contains(entry) {
			return false
		}
	}
	return true
}
