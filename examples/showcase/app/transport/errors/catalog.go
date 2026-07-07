package errors

import (
	xerrors "github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
)

// Catalog is the public HTTP error catalog for the showcase API.
var Catalog = xerrors.NewCatalog("showcase")

var (
	ErrUserNotFound = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindNotFound, Code: "10002", Message: "user not found", Source: "params.id",
	})
	ErrFetchUser = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindInternal, Code: "10003", Message: "fetch user failed",
	})
	ErrListUsers = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindInternal, Code: "10004", Message: "list users failed",
	})
	ErrListAudit = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindInternal, Code: "10006", Message: "list audit failed",
	})
	ErrUpdateUser = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindInternal, Code: "10007", Message: "update user failed",
	})
	ErrStale = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindConflict, Code: "10005", Message: "stale user version",
	})
	ErrValidation = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindValidation, Code: "10000", Message: "validation failed",
	})

	ErrDemoNotFound = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindNotFound, Code: "11004", Message: "item not found", Source: "params.id",
	})
	ErrDemoFetch = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindInternal, Code: "11005", Message: "fetch failed",
	})
	ErrDemoCreate = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindInternal, Code: "11008", Message: "create failed",
	})
	ErrDemoNameRequired = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindValidation, Code: "11002", Message: "name is required", Source: "body.name",
	})
	ErrDemoEmailInvalid = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindValidation, Code: "11003", Message: "email is invalid", Source: "body.email",
	})
	ErrUnauthorized = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindUnauthorized, Code: "11006", Message: "unauthorized",
	})
	ErrForbidden = Catalog.Define(xerrors.Def{
		Kind: xerrors.KindForbidden, Code: "11007", Message: "forbidden",
	})
)

// Fallbacks are registered once so repeated transport construction (e.g. tests)
// does not duplicate standard fallback codes on Catalog.
var Fallbacks = http.StandardErrors(Catalog)
