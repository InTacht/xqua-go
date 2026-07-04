package http

import (
	"strconv"

	"github.com/InTacht/xqua-go/pkg/errors"

	"github.com/gofiber/fiber/v3"
)

// ErrInvalidParam is the catalog-agnostic sentinel returned by the Param*
// helpers when a path parameter is missing or does not parse. It is a plain
// error by design: handlers map it into their own public catalog entry (with
// errors.Is / errors.Map), keeping the transport free of a status opinion.
//
//	id, err := http.ParamInt64(c, "id")
//	if err != nil {
//	    return apiErrInvalidID // or errors.Map(err, ...)
//	}
var ErrInvalidParam = errors.NewPlain("http: invalid path parameter")

// ParamInt64 reads path parameter name and parses it as a base-10 int64.
// A missing or unparseable value yields ErrInvalidParam.
func ParamInt64(c fiber.Ctx, name string) (int64, error) {
	v, err := strconv.ParseInt(c.Params(name), 10, 64)
	if err != nil {
		return 0, ErrInvalidParam
	}
	return v, nil
}

// ParamInt reads path parameter name and parses it as a base-10 int.
// A missing or unparseable value yields ErrInvalidParam.
func ParamInt(c fiber.Ctx, name string) (int, error) {
	v, err := strconv.Atoi(c.Params(name))
	if err != nil {
		return 0, ErrInvalidParam
	}
	return v, nil
}
