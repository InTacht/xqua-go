package errors

import stderrors "errors"

// Is reports whether err matches target, walking wrap chains and Errors
// collections via Error.Is and Errors.Is. It delegates to the standard library.
func Is(err, target error) bool {
	return stderrors.Is(err, target)
}

// Is reports whether target is the same catalog entry as this error on this
// node only: both must originate from the same Catalog.Define call (clones
// match their template). Kind and Code strings are never compared, so entries
// from different catalogs can never match each other. Depth is handled by the
// standard library via Unwrap.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok || e == nil || t == nil {
		return false
	}
	return e.entry != nil && e.entry == t.entry
}

// Is reports whether any collection member matches target. Each member is
// checked with the standard library errors.Is so wrapped entries are searched
// in depth as well as breadth.
func (e Errors) Is(target error) bool {
	for _, err := range e {
		if err != nil && Is(err, target) {
			return true
		}
	}
	return false
}

// IsKind reports whether any canonical error in err's tree has the given kind.
// Kinds are semantic categories shared freely across catalogs, so IsKind is
// the categorical complement to the exact-entry matching done by Is.
func IsKind(err error, kind string) bool {
	if err == nil || kind == "" {
		return false
	}

	switch e := err.(type) {
	case *Error:
		if e == nil {
			return false
		}
		if e.Kind == kind {
			return true
		}
		return IsKind(e.cause, kind)
	case Errors:
		for _, member := range e {
			if member != nil && IsKind(member, kind) {
				return true
			}
		}
		return false
	default:
		return IsKind(stderrors.Unwrap(err), kind)
	}
}
