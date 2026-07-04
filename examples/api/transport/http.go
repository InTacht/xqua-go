package transport

import (
	"context"
	"strconv"

	"github.com/InTacht/xqua-go/examples/api/store"
	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

// Public API catalog: the only errors allowed to cross the wire.
var apiCatalog = errors.NewCatalog("api")

var (
	errUserIDRequired = apiCatalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "10001", Message: "id is required", Source: "params.id",
	})
	errUserNotFound = apiCatalog.Define(errors.Def{
		Kind: errors.KindNotFound, Code: "10002", Message: "user not found", Source: "params.id",
	})
	errFetchUser = apiCatalog.Define(errors.Def{
		Kind: errors.KindInternal, Code: "10003", Message: "fetch user failed",
	})
	errListUsers = apiCatalog.Define(errors.Def{
		Kind: errors.KindInternal, Code: "10004", Message: "list users failed",
	})
)

// Deps is the subset of process state the HTTP unit needs. main narrows the
// app context at registration; this package never imports it.
type Deps struct {
	Host    string
	Port    int
	Version string
	Name    string
	Users   *store.Users
	Ping    func(context.Context) error
}

// HTTP builds the HTTP unit. Logger + Catalog are required; Host/Port come
// from env-backed app config.
func HTTP(d Deps, log runtime.Logger) runtime.Unit {
	return http.New(http.Config{
		Host:        d.Host,
		Port:        d.Port,
		Logger:      log,
		Catalog:     apiCatalog,
		HealthCheck: d.Ping,
		Version:     d.Version,
		FiberConfig: fiber.Config{ServerHeader: d.Name},
	}).
		Routes("/api/v1", func(r *http.Router) {
			r.Get("/users", listUsers(d.Users, log))
			r.Get("/users/:id", getUser(d.Users, log))
			r.Get("/boom", func(c fiber.Ctx) error {
				return errors.NewPlain("simulated failure")
			})
		})
}

func listUsers(s *store.Users, log runtime.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		users, err := s.List(c.Context(), 50)
		if err != nil {
			log.ErrorCtx(c.Context(), err, "list users failed")
			return mapStoreErr(err, errListUsers)
		}
		log.InfoCtx(c.Context(), "list users", strconv.Itoa(len(users)))
		return http.RES(c).
			Message("users listed").
			Data("users", users).
			Ok()
	}
}

func getUser(s *store.Users, log runtime.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, err := http.ParamInt64(c, "id")
		if err != nil || id <= 0 {
			return errUserIDRequired
		}

		user, err := s.GetByID(c.Context(), id)
		if err != nil {
			log.ErrorCtx(c.Context(), err, "fetch user failed", strconv.FormatInt(id, 10))
			return mapStoreErr(err, errFetchUser)
		}

		log.InfoCtx(c.Context(), "fetch user", strconv.FormatInt(id, 10))
		return http.RES(c).
			Message("user fetched").
			Data("user", user).
			Ok()
	}
}

// mapStoreErr maps internal store errors into the public API catalog.
func mapStoreErr(err error, fallback *errors.Error) error {
	return errors.MapOr(err, fallback,
		errors.Pair(store.ErrNotFound, errUserNotFound),
	)
}
