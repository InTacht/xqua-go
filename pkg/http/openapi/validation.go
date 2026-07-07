package openapi

import (
	"errors"
	"strings"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/validate"
)

// Severity represents the severity level of a validation error.
type Severity = validate.Severity

const (
	// SeverityError indicates a strict validation failure.
	SeverityError = validate.SeverityError
	// SeverityWarning indicates a validation warning that doesn't necessarily invalidate the document.
	SeverityWarning = validate.SeverityWarning
	// SeverityInfo indicates informational validation feedback.
	SeverityInfo = validate.SeverityInfo
)

// ValidationError represents a single validation error with an associated severity.
type ValidationError = validate.Error

// ValidationErrors is a collection of ValidationError. It implements the error
// interface and aggregates multiple validation issues into a single error value.
type ValidationErrors struct { //nolint:errname // ValidationErrors is a better name than ErrorsError or ValidationError here
	Errors []ValidationError
}

// Error implements the error interface by joining each entry with "; ".
func (e ValidationErrors) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, ve := range e.Errors {
		parts = append(parts, ve.Error())
	}
	return strings.Join(parts, "; ")
}

// Unwrap returns each ValidationError as a standard error for [errors.Is]/[errors.As] walks.
func (e ValidationErrors) Unwrap() []error {
	out := make([]error, 0, len(e.Errors))
	for i := range e.Errors {
		ve := e.Errors[i]
		out = append(out, &ve)
	}
	return out
}

// HasSeverity reports whether the collection contains an entry at the given severity.
func (e ValidationErrors) HasSeverity(s Severity) bool {
	for _, ve := range e.Errors {
		if ve.Severity == s {
			return true
		}
	}
	return false
}

// toValidationError converts an arbitrary error into a ValidationError. Non-
// validate.Error values are wrapped as SeverityError.
func toValidationError(err error) ValidationError {
	var valErr validate.Error
	var valErrPtr *validate.Error
	if errors.As(err, &valErrPtr) {
		return *valErrPtr
	}
	if errors.As(err, &valErr) {
		return valErr
	}
	return validate.Error{Err: err, Severity: SeverityError}
}

// collectValidationErrors converts a raw error slice into ValidationError entries,
// optionally filtered to a single severity.
func collectValidationErrors(errs []error, only Severity, filter bool) []ValidationError {
	var out []ValidationError
	for _, err := range errs {
		if err == nil {
			continue
		}
		ve := toValidationError(err)
		if filter && ve.Severity != only {
			continue
		}
		out = append(out, ve)
	}
	return out
}

// joinErrors returns a ValidationErrors containing only SeverityError entries,
// or nil when no strict failures exist.
func joinErrors(errs []error) error {
	filtered := collectValidationErrors(errs, SeverityError, true)
	if len(filtered) == 0 {
		return nil
	}
	return ValidationErrors{Errors: filtered}
}

// joinAllErrors returns a ValidationErrors containing every non-nil entry,
// preserving severity. Used by ValidateReport.
func joinAllErrors(errs []error) error {
	collected := collectValidationErrors(errs, 0, false)
	if len(collected) == 0 {
		return nil
	}
	return ValidationErrors{Errors: collected}
}
