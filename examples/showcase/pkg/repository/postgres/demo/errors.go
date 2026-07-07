package demo

import "github.com/InTacht/xqua-go/pkg/errors"

// Catalog holds internal demo-database repository errors.
var Catalog = errors.NewCatalog("demo")

var (
	// ErrQuery is returned when an audit query fails unexpectedly.
	ErrQuery = Catalog.Define(errors.Def{
		Kind: "internal", Code: "30001", Message: "audit query failed",
	})
)
