package validate_test

import (
	"errors"
	"testing"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/validate"
)

func TestErrorf(t *testing.T) {
	err := validate.Errorf("test error %d", 1)
	if err.Error() != "test error 1" {
		t.Errorf("expected 'test error 1', got %q", err.Error())
	}
	if err.Severity != validate.SeverityError {
		t.Errorf("expected SeverityError, got %v", err.Severity)
	}
}

func TestWarningf(t *testing.T) {
	err := validate.Warningf("test warning")
	if err.Error() != "test warning" {
		t.Errorf("expected 'test warning', got %q", err.Error())
	}
	if err.Severity != validate.SeverityWarning {
		t.Errorf("expected SeverityWarning, got %v", err.Severity)
	}
}

func TestInfof(t *testing.T) {
	err := validate.Infof("test info")
	if err.Error() != "test info" {
		t.Errorf("expected 'test info', got %q", err.Error())
	}
	if err.Severity != validate.SeverityInfo {
		t.Errorf("expected SeverityInfo, got %v", err.Severity)
	}
}

func TestErrorUnwrap(t *testing.T) {
	inner := errors.New("inner error")
	err := &validate.Error{Err: inner, Severity: validate.SeverityError}
	if !errors.Is(err, inner) {
		t.Error("expected err to wrap inner")
	}
	if !errors.Is(errors.Unwrap(err), inner) {
		t.Error("expected Unwrap to return inner")
	}
}
