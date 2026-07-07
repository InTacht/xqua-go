package users

import (
	"context"
	"strconv"
	"strings"

	"github.com/InTacht/xqua-go/examples/showcase/app/transport/errors"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository/postgres/core"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository/postgres/demo"
	usersvc "github.com/InTacht/xqua-go/examples/showcase/pkg/services/user"
	xerrors "github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/runtime"
)

type listUsersIn struct {
	Page int `query:"page" default:"1" minimum:"1"`
	Size int `query:"size" default:"20" minimum:"1" maximum:"100"`
}

type getUserIn struct {
	ID int64 `path:"id" minimum:"1"`
}

type updateUserIn struct {
	ID      int64  `path:"id" minimum:"1"`
	Name    string `json:"name" required:"true" example:"Updated User"`
	Email   string `json:"email" required:"true" example:"updated@example.com"`
	Version int    `json:"version" required:"true" example:"1"`
}

type listAuditIn struct {
	ID    int64 `path:"id" minimum:"1"`
	Limit int   `query:"limit" default:"20" minimum:"1" maximum:"100"`
}

type userOut struct {
	openapi.Response
	Data struct {
		User domain.User `json:"user"`
	} `json:"data"`
}

type usersOut struct {
	openapi.Response
	Data struct {
		Users []domain.User `json:"users"`
	} `json:"data"`
}

type auditOut struct {
	openapi.Response
	Data struct {
		Entries []domain.AuditEntry `json:"entries"`
	} `json:"data"`
}

type ackOut struct {
	openapi.Response
}

// Register mounts Postgres-backed user routes under /api/v1.
func Register(api *openapi.Generator, svc *usersvc.Service, log runtime.Logger) {
	api.Routes("/api/v1", func(r *openapi.Router) {
		v1 := r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().Err(422, errors.ErrValidation),
		})
		v1.Route("/users").Get(openapi.Route{
			Handler:   listUsers(svc, log),
			Summary:   "List users (paginated)",
			Responses: openapi.Returns().Err(500, errors.ErrListUsers),
		})
		v1.Route("/users/:id").Get(openapi.Route{
			Handler:   getUser(svc, log),
			Summary:   "Fetch one user",
			Responses: openapi.Returns().Err(404, errors.ErrUserNotFound).Err(500, errors.ErrFetchUser),
		})
		v1.Route("/users/:id").Put(openapi.Route{
			Handler: updateUser(svc, log),
			Summary: "Update one user",
			Responses: openapi.Returns().
				Err(404, errors.ErrUserNotFound).
				Err(409, errors.ErrStale).
				Err(500, errors.ErrUpdateUser),
		})
		v1.Route("/users/:id/audit").Get(openapi.Route{
			Handler:     listAudit(svc, log),
			Summary:     "List user audit trail",
			Description: "Reads audit rows from the demo Postgres database after verifying the user on core.",
			Responses: openapi.Returns().
				Err(404, errors.ErrUserNotFound).
				Err(500, errors.ErrListAudit),
		})
	})
}

func listUsers(svc *usersvc.Service, log runtime.Logger) func(context.Context, listUsersIn) (usersOut, error) {
	return func(ctx context.Context, in listUsersIn) (usersOut, error) {
		users, page, err := svc.ListPaged(ctx, in.Page, in.Size)
		if err != nil {
			log.ErrorCtx(ctx, err, "list users failed")
			return usersOut{}, mapCoreUserErr(err, errors.ErrListUsers)
		}
		log.InfoCtx(ctx, "list users", strconv.Itoa(len(users)))
		var out usersOut
		out.Message = "users listed"
		out.Data.Users = users
		out.Pagination = &page
		return out, nil
	}
}

func getUser(svc *usersvc.Service, log runtime.Logger) func(context.Context, getUserIn) (userOut, error) {
	return func(ctx context.Context, in getUserIn) (userOut, error) {
		user, err := svc.Get(ctx, in.ID)
		if err != nil {
			log.ErrorCtx(ctx, err, "fetch user failed", strconv.FormatInt(in.ID, 10))
			return userOut{}, mapCoreUserErr(err, errors.ErrFetchUser)
		}
		log.InfoCtx(ctx, "fetch user", strconv.FormatInt(in.ID, 10))
		var out userOut
		out.Message = "user fetched"
		out.Data.User = *user
		return out, nil
	}
}

func updateUser(svc *usersvc.Service, log runtime.Logger) func(context.Context, updateUserIn) (userOut, error) {
	return func(ctx context.Context, in updateUserIn) (userOut, error) {
		if in.Version != 1 {
			return userOut{}, errors.ErrStale
		}
		user, err := svc.Update(ctx, in.ID, strings.TrimSpace(in.Name), strings.TrimSpace(in.Email))
		if err != nil {
			log.ErrorCtx(ctx, err, "update user failed", strconv.FormatInt(in.ID, 10))
			return userOut{}, mapUpdateUserErr(err)
		}
		var out userOut
		out.Message = "user updated"
		out.Data.User = *user
		return out, nil
	}
}

func listAudit(svc *usersvc.Service, log runtime.Logger) func(context.Context, listAuditIn) (auditOut, error) {
	return func(ctx context.Context, in listAuditIn) (auditOut, error) {
		entries, err := svc.ListAudit(ctx, in.ID, in.Limit)
		if err != nil {
			log.ErrorCtx(ctx, err, "list audit failed", strconv.FormatInt(in.ID, 10))
			return auditOut{}, mapAuditErr(err)
		}
		var out auditOut
		out.Message = "audit listed"
		out.Data.Entries = entries
		return out, nil
	}
}

func mapCoreUserErr(err error, fallback *xerrors.Error) error {
	return xerrors.MapOr(err, fallback,
		xerrors.Pair(core.ErrNotFound, errors.ErrUserNotFound),
	)
}

func mapUpdateUserErr(err error) error {
	return xerrors.MapOr(err, errors.ErrUpdateUser,
		xerrors.Pair(core.ErrNotFound, errors.ErrUserNotFound),
		xerrors.Pair(core.ErrConflict, errors.ErrStale),
	)
}

func mapAuditErr(err error) error {
	return xerrors.MapOr(err, errors.ErrListAudit,
		xerrors.Pair(core.ErrNotFound, errors.ErrUserNotFound),
		xerrors.Pair(demo.ErrQuery, errors.ErrListAudit),
	)
}
