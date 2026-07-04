package store

import "github.com/InTacht/xqua-go/pkg/errors"

// Catalog is internal — it must never cross the HTTP wire.
var Catalog = errors.NewCatalog("store")

var (
	ErrNotFound = Catalog.Define(errors.Def{
		Kind: errors.KindNotFound, Code: "S001", Message: "item missing",
	})
	ErrCorrupt = Catalog.Define(errors.Def{
		Kind: errors.KindInternal, Code: "S002", Message: "corrupt row",
	})
)
