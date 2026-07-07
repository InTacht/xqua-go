package http

import (
	"github.com/InTacht/xqua-go/pkg/errors"
)

// StandardErrors defines conventional Unhandled and NotFound fallback entries
// on catalog and returns them as Fallbacks, so services stop hand-writing the
// two required fallbacks. Unhandled uses kind internal; NotFound uses kind
// not_found. Call once during catalog setup:
//
//	var API = errors.NewCatalog("api")
//	var fallbacks = http.StandardErrors(API)
func StandardErrors(catalog *errors.Catalog) Fallbacks {
	return Fallbacks{
		Unhandled: catalog.Define(errors.Def{
			Kind: errors.KindInternal, Code: "internal", Message: "internal error",
		}),
		NotFound: catalog.Define(errors.Def{
			Kind: errors.KindNotFound, Code: "not_found", Message: "route not found",
		}),
	}
}
