package surfaces

import (
	"context"

	"github.com/InTacht/xqua-go/examples/showcase/app/transport/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
)

type ackOut struct {
	openapi.Response
}

// Register mounts mobile and console surface stubs for multi-document OpenAPI demos.
func Register(api *openapi.Generator) {
	api.Routes("/mobile", func(r *openapi.Router) {
		v1 := r.Group(openapi.GroupConfig{Prefix: "/v1", Specs: []string{"mobile"}})
		v1.Route("/users/:id").Get(openapi.Route{
			Handler:   stubOK,
			Summary:   "Fetch one user (mobile)",
			Responses: openapi.Returns().Err(404, errors.ErrUserNotFound),
		})
		v1.Route("/users/manage").Post(openapi.Route{
			Handler:     stubOK,
			Summary:     "Manage a user",
			Description: "Activates or deactivates a user account.",
			Responses:   openapi.Returns().Err(409, errors.ErrStale),
			Requests: []openapi.ContentUnit{
				{Structure: &openapi.Schema{Ref: "#/components/schemas/ManageRequest"}, Required: true},
			},
		})
		r.Route("/ping").Get(openapi.Route{
			Handler:   stubOK,
			Summary:   "Mobile health ping",
			Specs:     []string{"shared"},
			Responses: openapi.Returns(),
		})
	})

	api.Routes("/console", func(r *openapi.Router) {
		r.Route("/v1/users").Get(openapi.Route{
			Handler:   stubOK,
			Summary:   "List users (console)",
			Responses: openapi.Returns(),
		})
		r.Route("/ping").Get(openapi.Route{
			Handler:   stubOK,
			Summary:   "Console health ping",
			Specs:     []string{"shared"},
			Responses: openapi.Returns(),
		})
	})
}

func stubOK(_ context.Context, _ struct{}) (ackOut, error) {
	return ackOut{Response: openapi.Response{Message: "ok"}}, nil
}
