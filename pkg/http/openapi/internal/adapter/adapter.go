package adapter

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/InTacht/xqua-go/pkg/errors"
	xhttp "github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/compile"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/wire"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

const (
	statusSuccess = "success"
	statusError   = "error"
)

// Handler returns a Fiber handler for a compiled route.
func Handler(route *compile.Route, catalog *errors.Catalog) fiber.Handler {
	return func(c fiber.Ctx) error {
		var in any
		var bindErr error
		if route.Binder != nil {
			in, bindErr = route.Binder.BindRequest(c)
		}
		if bindErr != nil {
			return handleHandlerError(c, bindErr, route, catalog)
		}
		if route.Call == nil {
			return nil
		}
		out, err := route.Call(c.Context(), in)
		if err != nil {
			return handleHandlerError(c, err, route, catalog)
		}
		return writeSuccess(c, out, route.Enveloped)
	}
}

func handleHandlerError(c fiber.Ctx, err error, route *compile.Route, catalog *errors.Catalog) error {
	entries := errors.AsErrors(err)
	if len(entries) == 0 {
		return err
	}
	for _, e := range entries {
		if !catalog.Contains(e) {
			return err
		}
	}
	status, ok := resolveDeclaredStatus(entries, route)
	if !ok {
		return err
	}
	return writeError(c, err, status)
}

func resolveDeclaredStatus(entries errors.Errors, route *compile.Route) (int, bool) {
	best := 0
	for _, e := range entries {
		st, ok := declaredStatus(e, route)
		if !ok {
			return 0, false
		}
		if st > best {
			best = st
		}
	}
	return best, true
}

func declaredStatus(e *errors.Error, route *compile.Route) (int, bool) {
	for _, c := range route.ErrCases {
		for _, tmpl := range c.Errors {
			if errors.Is(e, tmpl) {
				return c.Status, true
			}
		}
	}
	return 0, false
}

func writeSuccess(c fiber.Ctx, out any, enveloped bool) error {
	if enveloped {
		out = fillEnvelopedOut(out, requestid.FromContext(c), xhttp.ClientRequestIDFromContext(c))
	}
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Status(http.StatusOK).Send(data)
}

func fillEnvelopedOut(out any, requestID, clientRequestID string) any {
	v := reflect.ValueOf(out)
	if v.Kind() != reflect.Pointer {
		ptr := reflect.New(v.Type())
		ptr.Elem().Set(v)
		wire.FillSuccessEmbed(ptr.Interface(), requestID, clientRequestID)
		return ptr.Elem().Interface()
	}
	wire.FillSuccessEmbed(out, requestID, clientRequestID)
	return out
}

func writeError(c fiber.Ctx, err error, status int) error {
	entries := errors.AsErrors(err)
	if len(entries) == 0 {
		return err
	}
	details := make([]xhttp.ErrorDetail, 0, len(entries))
	for _, e := range entries {
		details = append(details, xhttp.ErrorDetailFrom(e))
	}
	env := wire.ErrorEnvelope{
		Status:          statusError,
		RequestID:       requestid.FromContext(c),
		ClientRequestID: xhttp.ClientRequestIDFromContext(c),
		Errors:          details,
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Status(status).Send(data)
}
