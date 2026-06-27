package transport

import (
	"time"

	"github.com/InTacht/xqua-go/examples/api/ctx"
	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/transport"
	"github.com/InTacht/xqua-go/pkg/transport/http"

	"github.com/gofiber/fiber/v3"
)

var (
	errUserIDRequired = errors.New("validation", "422301", "id is required", "params.id")
	errUserNotFound   = errors.New("not_found", "404301", "user not found", "params.id")
	errFetchUser      = errors.New("internal", "500301", "fetch user failed")
)

func HTTP(c *ctx.Ctx, log *logger.Logger) transport.Transport {
	return http.New(http.Config{
		Host:   c.Host,
		Port:   c.Port,
		Logger: log,
		FiberConfig: fiber.Config{
			ServerHeader: c.Name,
			ReadTimeout:  30 * time.Second,
		},
	}).
		Routes("/", func(r fiber.Router) {
			r.Get("/health", func(c fiber.Ctx) error {
				return http.RES(c).Message("alive").Ok()
			})
		}).
		Routes("/api/v1", func(r fiber.Router) {
			r.Get("/users/:id", getUser(log))
		})
}

func getUser(log *logger.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return http.RES(c).Message("validation failed").Error(errUserIDRequired).Ok()
		}

		user, err := fetchUser(id)
		if err != nil {
			err = mapFetchUserErr(err)
			if errors.Is(err, errUserNotFound) {
				return http.RES(c).Message("not found").Error(errUserNotFound).Ok()
			}
			log.ErrorCtx(c.Context(), err, "fetch user failed")
			return http.RES(c).Message("internal error").Apply(err).Ok()
		}

		log.InfoCtx(c.Context(), "fetch user", id)
		return http.RES(c).
			Message("user fetched").
			Data("user", user).
			Ok()
	}
}

func fetchUser(id string) (fiber.Map, error) {
	if id == "0" {
		return nil, errors.NewPlain("record not found")
	}

	return fiber.Map{
		"id":   id,
		"name": "Example User",
	}, nil
}

func mapFetchUserErr(err error) error {
	return errors.MapOr(err, errFetchUser.Kind, errFetchUser.Code, errFetchUser.Message, func(e error) (*errors.Error, bool) {
		if e.Error() == "record not found" {
			return errUserNotFound, true
		}
		return nil, false
	})
}
