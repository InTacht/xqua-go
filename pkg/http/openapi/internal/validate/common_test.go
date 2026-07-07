package validate_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/validate"
)

type GetUserRequest struct {
	ID string `path:"id" required:"true" description:"User identifier"`
}

type User struct {
	ID   string `json:"id" required:"true"`
	Name string `json:"name"`
}

func assertValidationContains(t *testing.T, err error, messages ...string) {
	t.Helper()
	require.Error(t, err)
	for _, message := range messages {
		assert.Contains(t, err.Error(), message)
	}
}

func assertHasError(t *testing.T, errs []error, message string) {
	t.Helper()
	for _, err := range errs {
		if strings.Contains(err.Error(), message) {
			return
		}
	}
	t.Errorf("expected error containing %q, got: %v", message, errs)
}

func assertHasWarning(t *testing.T, errs []error, message string) {
	t.Helper()
	for _, err := range errs {
		var valErr *validate.Error
		isWarning := errors.As(err, &valErr) && valErr.Severity == validate.SeverityWarning
		if isWarning && strings.Contains(err.Error(), message) {
			return
		}
	}
	t.Errorf("expected warning containing %q, got: %v", message, errs)
}

func assertNoStrictErrors(t *testing.T, errs []error) {
	t.Helper()
	for _, err := range errs {
		valErr := &validate.Error{}
		if errors.As(err, &valErr) {
			if valErr.Severity == validate.SeverityError {
				t.Errorf("unexpected strict error: %v", err)
			}
		} else {
			t.Errorf("unexpected standard error: %v", err)
		}
	}
}
