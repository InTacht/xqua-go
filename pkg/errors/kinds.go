package errors

// Standard kinds are the conventional semantic categories shared across
// catalogs. They are ordinary Kind strings: use them in Def.Kind so the openapi
// engine can resolve a sensible default status from the kind alone (see
// openapi.DefaultKindStatuses), while still allowing custom kinds and explicit
// per-route status overrides.
const (
	KindValidation   = "validation"
	KindNotFound     = "not_found"
	KindConflict     = "conflict"
	KindUnauthorized = "unauthorized"
	KindForbidden    = "forbidden"
	KindRateLimit    = "rate_limit"
	KindInternal     = "internal"
)
