package transport

import (
	"context"

	"github.com/InTacht/xqua-go/examples/showcase/app/transport/auth"
	"github.com/InTacht/xqua-go/examples/showcase/app/transport/errors"
	"github.com/InTacht/xqua-go/examples/showcase/app/transport/routes/demo"
	"github.com/InTacht/xqua-go/examples/showcase/app/transport/routes/surfaces"
	"github.com/InTacht/xqua-go/examples/showcase/app/transport/routes/users"
	authsvc "github.com/InTacht/xqua-go/examples/showcase/pkg/services/auth"
	itemsvc "github.com/InTacht/xqua-go/examples/showcase/pkg/services/item"
	usersvc "github.com/InTacht/xqua-go/examples/showcase/pkg/services/user"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

var manageRequestSchemas = map[string]*openapi.Schema{
	"ManageRequest": {
		Type:     "object",
		Required: []string{"action"},
		Properties: map[string]*openapi.Schema{
			"action": {Type: "string", Enum: []any{"activate", "deactivate"}},
			"note":   {Type: "string"},
		},
	},
}

var demoComponentSchemas = map[string]*openapi.Schema{
	"TokenChunk": {
		Type:     "object",
		Required: []string{"id", "delta"},
		Properties: map[string]*openapi.Schema{
			"id":    {Type: "string", Format: "uuid"},
			"delta": {Type: "string"},
		},
	},
	"ItemCreatedWebhook": {
		Type:     "object",
		Required: []string{"id", "name"},
		Properties: map[string]*openapi.Schema{
			"id":   {Type: "integer", Format: "int64"},
			"name": {Type: "string"},
		},
	},
}

var demoWebhooks = map[string]openapi.Webhook{
	"itemCreated": {
		Method: "POST",
		Route: openapi.Route{
			Summary: "Item created notification",
			Requests: []openapi.ContentUnit{{
				Required:    true,
				ContentType: "application/json",
				Structure:   &openapi.Schema{Ref: "#/components/schemas/ItemCreatedWebhook"},
			}},
			Responses: openapi.Returns(),
		},
	},
}

// Config is the HTTP transport configuration built by app.Run.
type Config struct {
	Host    string
	Port    int
	Version string
	Name    string
	Users   *usersvc.Service
	Items   *itemsvc.Service
	Auth    *authsvc.Service
	Ping    func(context.Context) error
}

// HTTP is the showcase transport: typed routes, OpenAPI docs, and catalog safety.
func HTTP(cfg Config, log runtime.Logger) runtime.Unit {
	t := http.New(http.Config{
		Host:        cfg.Host,
		Port:        cfg.Port,
		Logger:      log,
		Catalog:     errors.Catalog,
		Fallbacks:   errors.Fallbacks,
		HealthCheck: cfg.Ping,
		Version:     cfg.Version,
		FiberConfig: fiber.Config{ServerHeader: cfg.Name},
	})

	api := openapi.New(t, openapi.Config{
		Schemes: auth.Schemes(cfg.Auth),
		Specs: []openapi.Spec{
			{Path: "/openapi.json", Prefix: "/api/v1", Title: cfg.Name, Version: cfg.Version},
			{Path: "/mobile/openapi.json", Prefix: "/mobile", Title: "Mobile API", Version: cfg.Version, Schemas: manageRequestSchemas},
			{Path: "/console/openapi.json", Prefix: "/console", Title: "Console API", Version: cfg.Version},
			{
				Path:    "/mobile/v1/users/manage/openapi.json",
				Prefix:  "/mobile/v1/users/manage",
				Title:   "User Manage",
				Version: cfg.Version,
				Schemas: manageRequestSchemas,
			},
			{
				Path:     "/demo/openapi.json",
				Prefix:   "/demo",
				Title:    "Demo API",
				Version:  cfg.Version,
				Schemas:  demoComponentSchemas,
				Webhooks: demoWebhooks,
			},
		},
	})

	users.Register(api, cfg.Users, log)
	demo.Register(api, cfg.Items, t, log)
	auth.Register(api, cfg.Auth)
	surfaces.Register(api)

	return t
}
