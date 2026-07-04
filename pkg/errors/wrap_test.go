package errors_test

import (
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
)

func TestWrap(t *testing.T) {
	catalog := newTestCatalog()
	dbErr := errors.NewPlain("connection reset by peer")

	t.Run("preserves immediate cause", func(t *testing.T) {
		err := errors.Wrap(dbErr, catalog.errQueryFailed)
		if errors.Cause(err) != dbErr {
			t.Fatalf("expected cause %v, got %v", dbErr, errors.Cause(err))
		}
	})

	t.Run("AsErrors returns outermost catalog entry", func(t *testing.T) {
		err := errors.Wrap(dbErr, catalog.errQueryFailed)
		got := errors.AsErrors(err)
		if len(got) != 1 || got[0].Code != "500001" {
			t.Fatalf("unexpected errors: %+v", got)
		}
	})

	t.Run("nil cause returns catalog entry", func(t *testing.T) {
		if errors.Wrap(nil, catalog.errUserNotFound) != catalog.errUserNotFound {
			t.Fatal("Wrap(nil, catalog) should return catalog")
		}
	})

	t.Run("nil catalog returns cause", func(t *testing.T) {
		if errors.Wrap(dbErr, nil) != dbErr {
			t.Fatal("Wrap(cause, nil) should return cause")
		}
	})
}

func TestCause(t *testing.T) {
	catalog := newTestCatalog()
	dbErr := errors.NewPlain("connection reset by peer")

	t.Run("one level through nested wraps", func(t *testing.T) {
		repoErr := errors.Wrap(dbErr, catalog.errQueryFailed)
		serviceErr := errors.Wrap(repoErr, catalog.errFetchUserFailed)

		if errors.Cause(serviceErr) != repoErr {
			t.Fatalf("expected repoErr as immediate cause, got %v", errors.Cause(serviceErr))
		}
		if errors.Cause(repoErr) != dbErr {
			t.Fatalf("expected dbErr as immediate cause, got %v", errors.Cause(repoErr))
		}
	})

	t.Run("wrap over Errors collection", func(t *testing.T) {
		errs := errors.Errors{catalog.errIDRequired, catalog.errEmailInvalid}
		top := errors.Wrap(errs, catalog.errValidationFail)
		cause, ok := errors.Cause(top).(errors.Errors)
		if !ok || len(cause) != 2 {
			t.Fatalf("expected Errors as immediate cause, got %T", errors.Cause(top))
		}
	})

	t.Run("plain error returns nil", func(t *testing.T) {
		if errors.Cause(errors.NewPlain("plain")) != nil {
			t.Fatal("Cause(plain) should be nil")
		}
	})

	t.Run("nil returns nil", func(t *testing.T) {
		if errors.Cause(nil) != nil {
			t.Fatal("Cause(nil) should be nil")
		}
	})
}
