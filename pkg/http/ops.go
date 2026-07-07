package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// healthPath and versionPath are the operational endpoints registered
// automatically by every transport.
const (
	healthPath  = "/health"
	versionPath = "/version"
)

// installOps registers the built-in /health and /version endpoints. They are
// operational routes registered directly on Fiber and are never documented.
func (t *Transport) installOps() {
	t.app.Get(healthPath, t.handleHealth)
	t.app.Get(versionPath, t.handleVersion)
}

// handleHealth renders liveness. With no HealthCheck configured it is always
// alive (200). With one configured, a nil error is 200 "alive" and a non-nil
// error is a 503 "unavailable" envelope. Message and HTTP status carry the
// signal; no data payload.
func (t *Transport) handleHealth(c Ctx) error {
	if t.healthCheck == nil {
		return RES(c).Message("alive").Ok()
	}
	if err := t.healthCheck(c.Context()); err != nil {
		t.log.ErrorCtx(c.Context(), err, "health check failed")
		return writeEnvelope(c, fiber.StatusServiceUnavailable, envelope{
			Status:          statusError,
			Message:         "unavailable",
			RequestID:       requestid.FromContext(c),
			ClientRequestID: ClientRequestIDFromContext(c),
		})
	}
	return RES(c).Message("alive").Ok()
}

// handleVersion returns build information. Empty fields are omitted.
func (t *Transport) handleVersion(c Ctx) error {
	res := RES(c).Message("version")
	if t.build.Version != "" {
		res = res.Data("version", t.build.Version)
	}
	if t.build.BuildID != "" {
		res = res.Data("build_id", t.build.BuildID)
	}
	if t.build.BuildTime != "" {
		res = res.Data("build_time", t.build.BuildTime)
	}
	return res.Ok()
}

// writeEnvelope renders a pre-built envelope at an explicit HTTP status. It
// backs operational responses (like a degraded /health) that need a non-200
// status without going through the success/error status rules in Response.Ok.
func writeEnvelope(c Ctx, httpStatus int, out envelope) error {
	return c.Status(httpStatus).JSON(out)
}
