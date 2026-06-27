package errors

import stderrors "errors"

// Is reports whether err matches target, walking wrap chains and Errors
// collections via Error.Is and Errors.Is. It delegates to the standard library.
func Is(err, target error) bool {
	return stderrors.Is(err, target)
}

// Is reports whether target matches this catalog entry by kind and code on this
// node only. Depth is handled by the standard library via Unwrap.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok || e == nil || t == nil {
		return false
	}
	return e.Kind == t.Kind && e.Code == t.Code
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
