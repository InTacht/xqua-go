package transport

import (
	"context"

	"github.com/InTacht/xqua-go/examples/showcase/store"
	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

var apiCatalog = errors.NewCatalog("showcase")

var (
	errUserNotFound = apiCatalog.Define(errors.Def{
		Kind: errors.KindNotFound, Code: "10002", Message: "user not found", Source: "params.id",
	})
	errFetchUser = apiCatalog.Define(errors.Def{
		Kind: errors.KindInternal, Code: "10003", Message: "fetch user failed",
	})
	errListUsers = apiCatalog.Define(errors.Def{
		Kind: errors.KindInternal, Code: "10004", Message: "list users failed",
	})
	errStale = apiCatalog.Define(errors.Def{
		Kind: errors.KindConflict, Code: "10005", Message: "stale user version",
	})
	errValidation = apiCatalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "10000", Message: "validation failed",
	})

	errDemoNotFound = apiCatalog.Define(errors.Def{
		Kind: errors.KindNotFound, Code: "11004", Message: "item not found", Source: "params.id",
	})
	errDemoFetch = apiCatalog.Define(errors.Def{
		Kind: errors.KindInternal, Code: "11005", Message: "fetch failed",
	})
	errDemoNameRequired = apiCatalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "11002", Message: "name is required", Source: "body.name",
	})
	errDemoEmailInvalid = apiCatalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "11003", Message: "email is invalid", Source: "body.email",
	})
)

type Deps struct {
	Host    string
	Port    int
	Version string
	Name    string
	Users   *store.Users
	Ping    func(context.Context) error
}

// HTTP is the showcase transport: Postgres-backed users, in-memory demo routes,
// multi-surface OpenAPI, multipart upload, and catalog discipline examples.
func HTTP(d Deps, log runtime.Logger) runtime.Unit {
	t := http.New(http.Config{
		Host:        d.Host,
		Port:        d.Port,
		Logger:      log,
		Catalog:     apiCatalog,
		HealthCheck: d.Ping,
		Version:     d.Version,
		FiberConfig: fiber.Config{ServerHeader: d.Name},
	})

	api := openapi.New(t, openapi.Config{
		Specs: []openapi.Spec{
			{Path: "/openapi.json", Prefix: "/api/v1", Title: d.Name, Version: d.Version},
			{Path: "/mobile/openapi.json", Prefix: "/mobile", Title: "Mobile API", Version: d.Version},
			{Path: "/console/openapi.json", Prefix: "/console", Title: "Console API", Version: d.Version},
			{
				Path:    "/mobile/v1/users/manage/openapi.json",
				Prefix:  "/mobile/v1/users/manage",
				Title:   "User Manage",
				Version: d.Version,
				Schemas: map[string]*openapi.Schema{
					"ManageRequest": {
						Type:     "object",
						Required: []string{"action"},
						Properties: map[string]*openapi.Schema{
							"action": {Type: "string", Enum: []any{"activate", "deactivate"}},
							"note":   {Type: "string"},
						},
					},
				},
			},
			{Path: "/demo/openapi.json", Prefix: "/demo", Title: "Demo API", Version: d.Version},
		},
	})

	registerUserRoutes(api, d.Users, log)
	registerDemoRoutes(api, t, log)
	registerSurfaceRoutes(api)

	t.Fiber().Get("/api/v1/boom", func(c fiber.Ctx) error {
		return errors.NewPlain("simulated failure")
	})

	return t
}
