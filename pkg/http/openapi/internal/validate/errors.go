package validate

import "fmt"

// Severity represents the severity level of a validation error.
type Severity int

const (
	// SeverityError indicates a strict validation failure.
	SeverityError Severity = iota
	// SeverityWarning indicates a validation warning that doesn't necessarily invalidate the document.
	SeverityWarning
	// SeverityInfo indicates informational validation feedback.
	SeverityInfo
)

// Error represents a validation error with an associated severity level.
type Error struct {
	Err      error
	Severity Severity
}

// Error implements the error interface.
func (e Error) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error.
func (e Error) Unwrap() error {
	return e.Err
}

// Errorf creates a new validation error with SeverityError.
func Errorf(format string, args ...any) *Error {
	return &Error{
		Err:      fmt.Errorf(format, args...),
		Severity: SeverityError,
	}
}

// Warningf creates a new validation error with SeverityWarning.
func Warningf(format string, args ...any) *Error {
	return &Error{
		Err:      fmt.Errorf(format, args...),
		Severity: SeverityWarning,
	}
}

// Infof creates a new validation error with SeverityInfo.
func Infof(format string, args ...any) *Error {
	return &Error{
		Err:      fmt.Errorf(format, args...),
		Severity: SeverityInfo,
	}
}
