package errors

import (
	"strings"

	stderrors "errors"
)

// Error is the canonical error shape for xqua.
// Define kinds and codes in your application; the library only enforces shape.
// Wrapped errors form a linked list via cause for bubbling.
type Error struct {
	Kind    string
	Code    string
	Message string
	Source  string

	cause error
}

// New builds a canonical error from up to four positional strings:
// kind, code, message, source. Omitted trailing arguments are left empty.
func New(args ...string) *Error {
	if len(args) > 4 {
		args = args[:4]
	}
	e := &Error{}
	if len(args) > 0 {
		e.Kind = args[0]
	}
	if len(args) > 1 {
		e.Code = args[1]
	}
	if len(args) > 2 {
		e.Message = args[2]
	}
	if len(args) > 3 {
		e.Source = args[3]
	}
	return e
}

// NewPlain returns a plain error with the given message. It delegates to the
// standard library and is for ad-hoc or test errors that are not canonical errors.
func NewPlain(text string) error {
	return stderrors.New(text)
}

// Error implements the standard error interface.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	text := e.Kind

	if e.Code != "" {
		text += "<" + e.Code + ">"
	}

	if e.Source != "" {
		text += "(" + e.Source + ")"
	}

	if e.Message != "" {
		text += ": " + e.Message
	}

	return text
}

// Unwrap returns the next error in the chain.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// WithSource returns a copy with source set.
func (e *Error) WithSource(source string) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.Source = source
	return &cp
}

// WithMessage returns a copy with message set.
func (e *Error) WithMessage(message string) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.Message = message
	return &cp
}

// Errors is a collection of canonical errors.
type Errors []*Error

// Error implements the standard error interface for multiple canonical errors.
func (e Errors) Error() string {
	if len(e) == 0 {
		return ""
	}

	n := len(e)
	var text strings.Builder

	for idx, err := range e {
		if err != nil {
			text.WriteString(err.Error())

			if idx < n-1 {
				text.WriteString("\n")
			}
		}
	}

	return text.String()
}

// Append adds errors to the collection.
func (e Errors) Append(errs ...*Error) Errors {
	for _, err := range errs {
		if err != nil {
			e = append(e, err)
		}
	}
	return e
}

// AsErrors extracts canonical errors from err.
func AsErrors(err error) Errors {
	if err == nil {
		return nil
	}

	// If err is already a collection, return it.
	if es, ok := err.(Errors); ok && len(es) > 0 {
		return es
	}

	// If err is a canonical error, return a single-entry collection.
	var one *Error
	if stderrors.As(err, &one) && one != nil {
		return Errors{one}
	}

	// Otherwise, return nil.
	return nil
}

// IsStructured reports whether err carries the canonical error shape.
func IsStructured(err error) bool {
	return len(AsErrors(err)) > 0
}
