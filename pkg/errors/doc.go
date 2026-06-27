// Package errors defines the canonical error model for xqua-go services.
//
// # Overview
//
// Applications define their own error catalogs (kinds, codes, messages, sources).
// This package enforces shape and composition only—it does not ship predefined
// business errors.
//
// Two composition patterns cover the two common failure shapes:
//
//   - Vertical chains (Wrap): one failure path bubbling up through layers, e.g.
//     handler → service → repository → database driver error.
//   - Horizontal collections (Errors): several independent failures at the same
//     level, e.g. multiple validation field errors returned together.
//
// Nesting via Wrap does not replace Errors. A wrap chain implies causality
// ("this failed because of that"). A collection implies peers ("these all
// failed"). Do not chain independent validation errors with Wrap.
//
// Hybrid nesting combines both: an Errors sibling group may contain wrapped
// branches whose causes are further wraps or nested Errors collections:
//
//	siblings
//	-> error1 -> error2 -> siblings { error21, error22 }
//	-> error3 -> siblings { error4, error5 }
//	-> error6
//
// Build with Wrap (cause may be error, *Error chain, or Errors) and Errors
// for each sibling group.
//
// # Canonical shape
//
// Error is the single structured entry:
//
//   - Kind: coarse category (e.g. "validation", "not_found", "internal")
//   - Code: stable application identifier (e.g. "422301")
//   - Message: human-readable description
//   - Source: optional field path or origin (e.g. "body.id", "params.user_id")
//
// Error.Error renders kind, code, source, and message as a single string:
//
//	validation<422301>(body.id): id is required
//
// The underlying cause of a wrapped error is stored on an unexported field.
//
// # Catalog pattern
//
// Define shared catalog entries once, typically as package-level variables.
// New accepts up to four positional strings: kind, code, message, source.
// Omitted trailing arguments are left empty:
//
//	var errUserNotFound = errors.New("not_found", "404301", "user not found", "params.id")
//	var errQueryFailed = errors.New("internal", "500001", "query failed")
//
// NewPlain returns a plain error with the given message (stdlib errors.New).
// Use it for ad-hoc failures that are not canonical errors:
//
//	dbErr := errors.NewPlain("connection reset by peer")
//
// # Specializing catalog entries
//
// WithSource and WithMessage return a copy with one field changed. They do
// not mutate the original and do not attach a cause. There is no WithCause;
// use Wrap to attach an underlying failure at runtime.
//
// # Vertical chains: Wrap
//
// Wrap attaches a canonical error to an underlying cause:
//
//	repoErr := errors.Wrap(dbErr, errQueryFailed)
//	serviceErr := errors.Wrap(repoErr, errFetchUserFailed)
//
// Wrap clones the catalog entry, sets cause, and returns error (not *Error).
// Cause may be a plain error, another wrapped *Error, or an Errors collection.
// Cause returns the immediate underlying error. Error implements Unwrap for
// compatibility with the standard library.
//
// # Horizontal collections: Errors
//
// Errors is a slice of independent canonical errors:
//
//	return errors.Errors{errIDRequired, errEmailInvalid}
//
// Use Append to build collections incrementally. Errors.Error joins member
// strings with newlines.
//
// # Converting plain errors: Map, Or, MapOr
//
// Use at layer boundaries when a lower level returns a plain error:
//
//   - Map runs Mapper functions until one recognizes the error, then Wraps the
//     original as cause under the matched catalog entry.
//   - Or Wraps any unstructured error with a fallback catalog entry (message
//     defaults to err.Error() when empty). Source is not set by Or.
//   - MapOr tries Map first, then falls back to Or.
//
// # Matching
//
// Is reports whether err matches target, walking wrap chains and Errors
// collections. Error.Is compares kind and code on the current node only;
// Errors.Is checks each member. Message and source are ignored when matching
// catalog identity:
//
//	if errors.Is(err, errUserNotFound) { ... }
//
// Is also finds plain errors nested in wrap chains by identity.
//
// # Extraction
//
// AsErrors extracts canonical error(s) from err. For a hybrid tree it returns
// top-level branch heads only; for a single wrapped entry it returns that entry.
// IsStructured reports whether AsErrors would return a non-empty result.
//
// # Quick reference
//
//	Operation                    Use when
//	---------                    --------
//	New                          Define a catalog entry (kind, code, message, source)
//	NewPlain                     Ad-hoc plain error (stdlib errors.New)
//	WithSource / WithMessage     Specialize catalog fields (no cause)
//	Wrap                         Attach cause; bubble up one layer
//	Cause                        Immediate underlying error
//	Errors / Append              Return multiple independent failures
//	Map / Or / MapOr             Convert plain errors at boundaries
//	Is                           Match catalog or plain errors in hybrid trees
//	AsErrors / IsStructured      Extract canonical error(s) from err
package errors
