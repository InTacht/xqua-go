package core

import "github.com/InTacht/xqua-go/pkg/errors"

// Catalog holds internal core-database repository errors.
var Catalog = errors.NewCatalog("core")

var (
	// ErrNotFound is returned when a user row does not exist.
	ErrNotFound = Catalog.Define(errors.Def{
		Kind: "not_found", Code: "20001", Message: "user not found",
	})
	// ErrQuery is returned when a database operation fails unexpectedly.
	ErrQuery = Catalog.Define(errors.Def{
		Kind: "internal", Code: "20002", Message: "user query failed",
	})
	// ErrConflict is returned when an update violates a uniqueness constraint.
	ErrConflict = Catalog.Define(errors.Def{
		Kind: "conflict", Code: "20003", Message: "user conflict",
	})
)
