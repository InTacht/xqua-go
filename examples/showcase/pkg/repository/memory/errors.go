package memory

import "github.com/InTacht/xqua-go/pkg/errors"

// Catalog holds internal in-memory repository errors.
var Catalog = errors.NewCatalog("memory")

var (
	ErrItemNotFound = Catalog.Define(errors.Def{
		Kind: errors.KindNotFound, Code: "M001", Message: "item missing",
	})
	ErrItemCorrupt = Catalog.Define(errors.Def{
		Kind: errors.KindInternal, Code: "M002", Message: "corrupt row",
	})
	ErrIssueToken = Catalog.Define(errors.Def{
		Kind: errors.KindInternal, Code: "M003", Message: "issue token failed",
	})
)
