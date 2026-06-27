package errors

import stderrors "errors"

// Wrap attaches a canonical error to an underlying cause.
// Cause may be a plain error, another wrapped *Error, or an Errors collection
// (hybrid nesting: vertical chains with sibling groups at any depth).
func Wrap(cause error, err *Error) error {
	if cause == nil {
		return err
	}
	if err == nil {
		return cause
	}
	cp := clone(err)
	cp.cause = cause
	return cp
}

// Cause returns the immediate underlying error when err was produced by Wrap.
func Cause(err error) error {
	if e, ok := stderrors.AsType[*Error](err); ok {
		return e.cause
	}
	return nil
}

func clone(err *Error) *Error {
	if err == nil {
		return nil
	}
	cp := *err
	cp.cause = nil
	return &cp
}
