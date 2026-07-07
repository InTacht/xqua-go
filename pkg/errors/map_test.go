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

	t.Run("structured input passes through when no mapper matches", func(t *testing.T) {
		original := catalog.errIDRequired
		if errors.Map(original, func(err error) (*errors.Error, bool) {
			return nil, false
		}) != original {
			t.Fatal("Map should pass structured errors through when unmatched")
		}
	})

	t.Run("structured input can be re-mapped at boundaries", func(t *testing.T) {
		internal := catalog.errUserNotFound
		mapped := errors.Map(internal, func(err error) (*errors.Error, bool) {
			if errors.Is(err, catalog.errUserNotFound) {
				return catalog.errFetchUserFailed, true
			}
			return nil, false
		})
		if !errors.Is(mapped, catalog.errFetchUserFailed) {
			t.Fatal("expected mapper result to win over structured pass-through")
		}
		if !errors.Is(mapped, catalog.errUserNotFound) {
			t.Fatal("expected original error preserved as cause")
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
		err := errors.Or(cause, catalog.errFallback)
		got := errors.AsErrors(err)
		if len(got) != 1 {
			t.Fatalf("expected 1 error, got %d", len(got))
		}
		if got[0].Message != "fallback message" {
			t.Fatalf("expected catalog message, got %q", got[0].Message)
		}
		if errors.Cause(err) != cause {
			t.Fatal("expected cause preserved")
		}
	})

	t.Run("structured input passes through", func(t *testing.T) {
		original := catalog.errIDRequired
		if errors.Or(original, catalog.errFallback) != original {
			t.Fatal("Or should pass structured errors through unchanged")
		}
	})

	t.Run("nil returns nil", func(t *testing.T) {
		if errors.Or(nil, catalog.errFallback) != nil {
			t.Fatal("Or(nil) should be nil")
		}
	})

	t.Run("nil fallback panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for nil fallback")
			}
		}()
		errors.Or(errors.NewPlain("boom"), nil)
	})
}

func TestMapOr(t *testing.T) {
	catalog := newTestCatalog()

	t.Run("uses mapper when it matches", func(t *testing.T) {
		err := errors.MapOr(
			errors.NewPlain("missing"),
			catalog.errFallback,
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
			catalog.errFallback,
			func(err error) (*errors.Error, bool) { return nil, false },
		)
		if errors.AsErrors(err)[0].Code != "500000" {
			t.Fatal("MapOr should fall back to catalog fallback")
		}
	})

	t.Run("wraps unmatched structured error with fallback", func(t *testing.T) {
		internal := catalog.errIDRequired
		err := errors.MapOr(
			internal,
			catalog.errFallback,
			errors.Pair(catalog.errUserNotFound, catalog.errFetchUserFailed),
		)
		if !errors.Is(err, catalog.errFallback) {
			t.Fatal("MapOr should wrap unmatched structured errors with fallback")
		}
		if !errors.Is(err, internal) {
			t.Fatal("expected original error preserved in wrap chain")
		}
	})

	t.Run("nil returns nil", func(t *testing.T) {
		if errors.MapOr(nil, catalog.errFallback) != nil {
			t.Fatal("MapOr(nil) should be nil")
		}
	})
}

func TestPair(t *testing.T) {
	catalog := newTestCatalog()

	t.Run("maps matching error to target", func(t *testing.T) {
		mapper := errors.Pair(catalog.errUserNotFound, catalog.errFetchUserFailed)
		got, ok := mapper(catalog.errUserNotFound)
		if !ok || got != catalog.errFetchUserFailed {
			t.Fatalf("expected match to fetch-user error, got %v ok=%v", got, ok)
		}
	})

	t.Run("matches through wrap chain", func(t *testing.T) {
		mapper := errors.Pair(catalog.errUserNotFound, catalog.errFetchUserFailed)
		wrapped := errors.Wrap(errors.NewPlain("boom"), catalog.errUserNotFound)
		if _, ok := mapper(wrapped); !ok {
			t.Fatal("expected Pair to match wrapped clone by identity")
		}
	})

	t.Run("no match returns false", func(t *testing.T) {
		mapper := errors.Pair(catalog.errUserNotFound, catalog.errFetchUserFailed)
		if _, ok := mapper(catalog.errIDRequired); ok {
			t.Fatal("expected no match for a different entry")
		}
	})
}

func TestMappers(t *testing.T) {
	catalog := newTestCatalog()

	composed := errors.Mappers(
		errors.Pair(catalog.errUserNotFound, catalog.errFetchUserFailed),
		errors.Pair(catalog.errIDRequired, catalog.errFallback),
	)

	t.Run("consults in order and returns first match", func(t *testing.T) {
		if got, ok := composed(catalog.errIDRequired); !ok || got != catalog.errFallback {
			t.Fatalf("expected second mapper to match, got %v ok=%v", got, ok)
		}
	})

	t.Run("composes cleanly into MapOr", func(t *testing.T) {
		err := errors.MapOr(catalog.errUserNotFound, catalog.errFallback, composed)
		if !errors.Is(err, catalog.errFetchUserFailed) {
			t.Fatal("expected composed mapper result via MapOr")
		}
	})

	t.Run("no match returns false", func(t *testing.T) {
		if _, ok := composed(errors.NewPlain("unknown")); ok {
			t.Fatal("expected no match")
		}
	})
}
