// Package errors defines the catalog-driven error model for xqua-go services.
//
// # Overview
//
// Errors are defined upfront in a Catalog, one per module or logical space.
// Identity is the defined entry itself: Is matches only errors that originate
// from the same Catalog.Define call, so entries from different catalogs can
// never collide — even when they share Kind or Code strings. There is no
// global registry; catalogs are plain values, immutable after package init.
//
// Kind is a semantic category ("validation", "not_found", ...) shared freely
// across catalogs and matched with IsKind. It defaults to the catalog name.
// The standard kind constants (KindValidation, KindNotFound, KindConflict,
// KindUnauthorized, KindForbidden, KindRateLimit, KindInternal) are the
// conventional categories the openapi engine can resolve default statuses from
// (see openapi.DefaultKindStatuses); custom kinds remain fully supported.
//
// The package is transport-agnostic. HTTP status codes are decided where an
// error is surfaced (route mappings), not on the error itself. Only a
// service's designated public catalog crosses the wire; internal catalog
// errors are mapped at the boundary (see Map, Or, MapOr).
//
// Two composition patterns cover the two common failure shapes:
//
//   - Vertical chains (Wrap): one failure path bubbling up through layers.
//   - Horizontal collections (Errors): several independent failures at once.
//
// # Catalog
//
//	// One catalog per module/space; Kind defaults to the catalog name.
//	var Store = errors.NewCatalog("store")
//
//	var (
//	    ErrUserMissing = Store.Define(errors.Def{
//	        Kind: "not_found", Code: "10001", Message: "user not found", Source: "id",
//	    })
//	    ErrConflict = Store.Define(errors.Def{Code: "10002", Message: "stale version"}),
//	)
//
// Define panics on an empty or duplicate Code within a catalog. Lookup decodes
// a wire code back to its entry; Entries enumerates a catalog for manifests,
// OpenAPI schemas, or client code generation; Contains reports membership.
//
// # Canonical shape
//
// Error fields: Kind (semantic category), Code (unique per catalog), Message,
// Source. Clones made by Wrap, WithSource, and WithMessage keep the identity
// of their template entry.
//
// NewPlain returns a plain error for underlying causes (driver, stdlib). Use
// Map, Or, or MapOr to convert plain errors to catalog entries at boundaries.
//
// # Converting errors at boundaries: Map, Or, MapOr
//
//   - Map runs Mapper functions until one recognizes the error. Mappers run
//     first even for structured errors, so internal catalog entries can be
//     re-mapped to public catalog entries (match with Is inside the mapper).
//     Unmatched structured errors pass through unchanged.
//   - Or wraps unstructured errors with a catalog fallback entry. Structured
//     errors pass through unchanged.
//   - MapOr tries mappers first, then wraps with the fallback when none match —
//     including for structured internal catalog errors at HTTP/store boundaries.
//
// Pair and Mappers are combinators that make boundary mapping declarative:
// Pair(from, to) is a Mapper that translates errors matching from (by Is) to
// to, and Mappers(...) composes several into one.
//
//	return errors.MapOr(err, api.ErrInternal,
//	    errors.Pair(store.ErrNotFound, api.ErrUserNotFound),
//	    errors.Pair(store.ErrConflict, api.ErrConflict),
//	)
//
// # Matching and extraction
//
//   - Is: exact entry identity, walking wrap chains and Errors collections.
//   - IsKind: categorical matching by Kind anywhere in the tree.
//   - AsErrors: extracts catalog entries for logging or transport encoding.
package errors
