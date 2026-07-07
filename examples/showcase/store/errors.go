package store

import "github.com/InTacht/xqua-go/pkg/errors"

// Catalog holds internal store errors. They must be mapped into the service's
// public API catalog before crossing the HTTP boundary.
var Catalog = errors.NewCatalog("store")

var (
	// ErrNotFound is returned when a user row does not exist.
	ErrNotFound = Catalog.Define(errors.Def{
		Kind: "not_found", Code: "20001", Message: "user not found",
	})
	// ErrQuery is returned when a database operation fails unexpectedly.
	ErrQuery = Catalog.Define(errors.Def{
		Kind: "internal", Code: "20002", Message: "user query failed",
	})
)
