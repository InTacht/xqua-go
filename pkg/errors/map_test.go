package errors_test

import (
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
)

func TestMap(t *testing.T) {
	catalog := newTestCatalog()

	t.Run("recognized plain error", func(t *testing.T) {
		plain := errors.NewPlain("record not found")
		mapped := errors.Map(plain, func(err error) (*errors.Error, bool) {
			if err.Error() == "record not found" {
				return catalog.errUserNotFound, true
			}
			return nil, false
		})
		if !errors.Is(mapped, catalog.errUserNotFound) {
			t.Fatal("expected mapped error to match catalog")
		}
		if errors.Cause(mapped) != plain {
			t.Fatal("expected cause preserved")
		}
	})

	t.Run("no mapper match returns nil", func(t *testing.T) {
		if errors.Map(errors.NewPlain("unknown"), func(err error) (*errors.Error, bool) {
			return nil, false
		}) != nil {
			t.Fatal("expected nil when no mapper matches")
		}
	})

	t.Run("structured input passes through", func(t *testing.T) {
		original := catalog.errIDRequired
		if errors.Map(original, func(err error) (*errors.Error, bool) {
			return catalog.errUserNotFound, true
		}) != original {
			t.Fatal("Map should pass structured errors through unchanged")
		}
	})

	t.Run("nil returns nil", func(t *testing.T) {
		if errors.Map(nil) != nil {
			t.Fatal("Map(nil) should be nil")
		}
	})
}

func TestOr(t *testing.T) {
	catalog := newTestCatalog()

	t.Run("wraps plain error with fallback catalog entry", func(t *testing.T) {
		cause := errors.NewPlain("boom")
		err := errors.Or(cause, "internal", "500000", "")
		got := errors.AsErrors(err)
		if len(got) != 1 {
			t.Fatalf("expected 1 error, got %d", len(got))
		}
		if got[0].Message != "boom" {
			t.Fatalf("expected cause message, got %q", got[0].Message)
		}
		if got[0].Error() != "internal<500000>: boom" {
			t.Fatalf("unexpected Error() string: %q", got[0].Error())
		}
	})

	t.Run("structured input passes through", func(t *testing.T) {
		original := catalog.errIDRequired
		if errors.Or(original, "internal", "500000", "ignored") != original {
			t.Fatal("Or should pass structured errors through unchanged")
		}
	})

	t.Run("nil returns nil", func(t *testing.T) {
		if errors.Or(nil, "internal", "500000", "ignored") != nil {
			t.Fatal("Or(nil) should be nil")
		}
	})
}

func TestMapOr(t *testing.T) {
	catalog := newTestCatalog()

	t.Run("uses mapper when it matches", func(t *testing.T) {
		err := errors.MapOr(
			errors.NewPlain("missing"),
			"internal", "500000", "fallback message",
			func(err error) (*errors.Error, bool) {
				if err.Error() == "missing" {
					return catalog.errIDRequired, true
				}
				return nil, false
			},
		)
		if !errors.Is(err, catalog.errIDRequired) {
			t.Fatal("MapOr should use mapper result")
		}
	})

	t.Run("falls back to Or when mapper misses", func(t *testing.T) {
		err := errors.MapOr(
			errors.NewPlain("unexpected"),
			"internal", "500000", "fallback message",
			func(err error) (*errors.Error, bool) { return nil, false },
		)
		if errors.AsErrors(err)[0].Code != "500000" {
			t.Fatal("MapOr should fall back to Or")
		}
	})

	t.Run("nil returns nil", func(t *testing.T) {
		if errors.MapOr(nil, "internal", "500000", "fallback") != nil {
			t.Fatal("MapOr(nil) should be nil")
		}
	})
}
