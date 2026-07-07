package transport

import (
	"context"
	"strconv"

	"github.com/InTacht/xqua-go/examples/showcase/store"
	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/runtime"
)

type listUsersIn struct{}

type getUserIn struct {
	ID int64 `path:"id" minimum:"1"`
}

type userOut struct {
	openapi.Response
	Data struct {
		User store.User `json:"user"`
	} `json:"data"`
}

type usersOut struct {
	openapi.Response
	Data struct {
		Users []store.User `json:"users"`
	} `json:"data"`
}

type ackOut struct {
	openapi.Response
}

func registerUserRoutes(api *openapi.Generator, users *store.Users, log runtime.Logger) {
	api.Routes("/api/v1", func(r *openapi.Router) {
		v1 := r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().Err(422, errValidation),
		})
		v1.Route("/users").Get(openapi.Route{
			Handler:   listUsers(users, log),
			Summary:   "List users",
			Responses: openapi.Returns().Err(500, errListUsers),
		})
		v1.Route("/users/:id").Get(openapi.Route{
			Handler:   getUser(users, log),
			Summary:   "Fetch one user",
			Responses: openapi.Returns().Err(404, errUserNotFound).Err(500, errFetchUser),
		})
	})
}

func registerSurfaceRoutes(api *openapi.Generator) {
	api.Routes("/mobile", func(r *openapi.Router) {
		v1 := r.Group(openapi.GroupConfig{Prefix: "/v1", Specs: []string{"mobile"}})
		v1.Route("/users/:id").Get(openapi.Route{
			Handler:   stubOK,
			Summary:   "Fetch one user (mobile)",
			Responses: openapi.Returns().Err(404, errUserNotFound),
		})
		v1.Route("/users/manage").Post(openapi.Route{
			Handler:     stubOK,
			Summary:     "Manage a user",
			Description: "Activates or deactivates a user account.",
			Responses:   openapi.Returns().Err(409, errStale),
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

func listUsers(s *store.Users, log runtime.Logger) func(context.Context, listUsersIn) (usersOut, error) {
	return func(ctx context.Context, _ listUsersIn) (usersOut, error) {
		users, err := s.List(ctx, 50)
		if err != nil {
			log.ErrorCtx(ctx, err, "list users failed")
			return usersOut{}, mapStoreErr(err, errListUsers)
		}
		log.InfoCtx(ctx, "list users", strconv.Itoa(len(users)))
		var out usersOut
		out.Message = "users listed"
		out.Data.Users = users
		return out, nil
	}
}

func getUser(s *store.Users, log runtime.Logger) func(context.Context, getUserIn) (userOut, error) {
	return func(ctx context.Context, in getUserIn) (userOut, error) {
		user, err := s.GetByID(ctx, in.ID)
		if err != nil {
			log.ErrorCtx(ctx, err, "fetch user failed", strconv.FormatInt(in.ID, 10))
			return userOut{}, mapStoreErr(err, errFetchUser)
		}
		log.InfoCtx(ctx, "fetch user", strconv.FormatInt(in.ID, 10))
		var out userOut
		out.Message = "user fetched"
		out.Data.User = *user
		return out, nil
	}
}

func mapStoreErr(err error, fallback *errors.Error) error {
	return errors.MapOr(err, fallback,
		errors.Pair(store.ErrNotFound, errUserNotFound),
	)
}
