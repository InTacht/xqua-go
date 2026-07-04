package http

import (
	"fmt"
	"time"

	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// HeaderClientRequestID is the header a caller uses to supply its own
// correlation ID. A valid value is echoed back on the response and included in
// the envelope as client_request_id.
const HeaderClientRequestID = "X-Client-Request-Id"

// maxClientRequestIDLen bounds echoed client IDs so arbitrary long input is
// not reflected into response headers.
const maxClientRequestIDLen = 128

type clientRequestIDKeyType struct{}

var clientRequestIDKey = clientRequestIDKeyType{}

// RequestContext bridges Fiber request IDs into the standard logger context.
func RequestContext() fiber.Handler {
	return func(c fiber.Ctx) error {
		if id := requestid.FromContext(c); id != "" {
			c.SetContext(logger.ContextWithRequestID(c.Context(), id))
		}
		return c.Next()
	}
}

// ClientRequestID echoes a valid X-Client-Request-Id request header back on
// the response and stores it in context for the RES envelope.
func ClientRequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		if id := c.Get(HeaderClientRequestID); isValidClientRequestID(id) {
			c.Set(HeaderClientRequestID, id)
			fiber.StoreInContext(c, clientRequestIDKey, id)
		}
		return c.Next()
	}
}

// ClientRequestIDFromContext returns the caller-supplied correlation ID stored
// by the ClientRequestID middleware, or "" when absent.
func ClientRequestIDFromContext(ctx any) string {
	if id, ok := fiber.ValueFromContext[string](ctx, clientRequestIDKey); ok {
		return id
	}
	return ""
}

// isValidClientRequestID accepts non-empty, bounded, visible-ASCII values so
// client input can be safely reflected into a response header.
func isValidClientRequestID(id string) bool {
	if id == "" || len(id) > maxClientRequestIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] < 0x20 || id[i] > 0x7e {
			return false
		}
	}
	return true
}

// AccessLog logs one line per request using the transport logger. The log
// level is status-aware — 5xx logs at error, 4xx at warn, and everything else
// at info — and the noisy /health endpoint is skipped.
func AccessLog(log runtime.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Path() == healthPath {
			return c.Next()
		}
		start := time.Now()
		err := c.Next()

		status := c.Response().StatusCode()
		detail := fmt.Sprintf("status=%d latency=%s", status, time.Since(start))
		ctx := c.Context()
		switch {
		case status >= 500:
			log.ErrorCtx(ctx, err, c.Method(), c.Path(), detail)
		case status >= 400:
			log.WarnCtx(ctx, c.Method(), c.Path(), detail)
		default:
			log.InfoCtx(ctx, c.Method(), c.Path(), detail)
		}
		return err
	}
}
