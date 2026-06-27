package errors_test

import (
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
)

func TestIsCatalog(t *testing.T) {
	catalog := newTestCatalog()
	dbErr := errors.NewPlain("sql: no rows")

	t.Run("matches kind and code only", func(t *testing.T) {
		err := errors.Wrap(dbErr, catalog.errUserNotFound)
		if !errors.Is(err, catalog.errUserNotFound) {
			t.Fatal("expected Is to match catalog")
		}
		if errors.Is(err, errors.New("not_found", "404302", "order not found", "params.id")) {
			t.Fatal("expected different code to not match")
		}
		if errors.Is(err, errors.New("validation", "404301", "wrong kind")) {
			t.Fatal("expected different kind to not match")
		}
	})

	t.Run("finds entries anywhere in vertical chain", func(t *testing.T) {
		err := errors.Wrap(dbErr, catalog.errUserNotFound)
		repoErr := errors.Wrap(err, catalog.errQueryFailed)
		serviceErr := errors.Wrap(repoErr, catalog.errFetchUserFailed)

		checks := []struct {
			label  string
			target *errors.Error
			want   bool
		}{
			{"outer", catalog.errFetchUserFailed, true},
			{"middle", catalog.errQueryFailed, true},
			{"inner", catalog.errUserNotFound, true},
			{"same code wrong kind", errors.New("validation", "500002", "wrong kind"), false},
			{"same kind wrong code", errors.New("internal", "500999", "wrong code"), false},
			{"different message same catalog", catalog.errFetchUserFailed.WithMessage("timeout"), true},
		}

		for _, tc := range checks {
			t.Run(tc.label, func(t *testing.T) {
				if got := errors.Is(serviceErr, tc.target); got != tc.want {
					t.Fatalf("Is(%s): got %v, want %v", tc.label, got, tc.want)
				}
			})
		}
	})

	t.Run("matches members of Errors collection", func(t *testing.T) {
		errs := errors.Errors{catalog.errIDRequired, catalog.errEmailInvalid}
		if !errors.Is(errs, catalog.errIDRequired) {
			t.Fatal("expected Is to match collection member")
		}
		if errors.Is(errs, catalog.errUserNotFound) {
			t.Fatal("expected non-member to not match")
		}
	})

	t.Run("Errors Is skips nil members", func(t *testing.T) {
		errs := errors.Errors{nil, catalog.errIDRequired}
		if !errors.Is(errs, catalog.errIDRequired) {
			t.Fatal("expected Is to match non-nil member")
		}
	})

	t.Run("depth inside breadth", func(t *testing.T) {
		deep := errors.Wrap(errors.NewPlain("connection reset"), catalog.errQueryFailed).(*errors.Error)
		errs := errors.Errors{deep, catalog.errIDRequired}
		if !errors.Is(errs, catalog.errQueryFailed) {
			t.Fatal("expected Is to find catalog entry inside wrapped collection member")
		}
	})

	t.Run("breadth inside depth", func(t *testing.T) {
		errs := errors.Errors{catalog.errIDRequired, catalog.errEmailInvalid}
		top := errors.Wrap(errs, errors.New("internal", "500000", "validation failed"))
		if !errors.Is(top, catalog.errEmailInvalid) {
			t.Fatal("expected Is to find catalog entry inside Errors attached as cause")
		}
	})

	t.Run("plain and nil inputs", func(t *testing.T) {
		if errors.Is(errors.NewPlain("plain"), catalog.errUserNotFound) {
			t.Fatal("Is(plain, catalog) should be false")
		}
		if errors.Is(nil, catalog.errUserNotFound) {
			t.Fatal("Is(nil, catalog) should be false")
		}
	})
}

func TestIsPlain(t *testing.T) {
	catalog := newTestCatalog()
	dbErr := errors.NewPlain("connection reset by peer")

	t.Run("nested in vertical chain", func(t *testing.T) {
		repoErr := errors.Wrap(dbErr, catalog.errQueryFailed)
		serviceErr := errors.Wrap(repoErr, catalog.errFetchUserFailed)

		if !errors.Is(serviceErr, dbErr) {
			t.Fatal("expected Is to find plain error nested in wrap chain")
		}
		if !errors.Is(repoErr, dbErr) {
			t.Fatal("expected Is to find plain error one wrap layer down")
		}
		if errors.Is(serviceErr, errors.NewPlain("connection reset by peer")) {
			t.Fatal("expected Is to match by identity, not message")
		}
	})

	t.Run("inside Errors collection", func(t *testing.T) {
		errs := errors.Errors{
			errors.Wrap(dbErr, catalog.errQueryFailed).(*errors.Error),
			catalog.errIDRequired,
		}
		if !errors.Is(errs, dbErr) {
			t.Fatal("expected Is to find plain error inside Errors collection member")
		}
	})

	t.Run("through wrap into Errors collection", func(t *testing.T) {
		errs := errors.Errors{
			errors.Wrap(dbErr, catalog.errQueryFailed).(*errors.Error),
			catalog.errIDRequired,
		}
		top := errors.Wrap(errs, errors.New("internal", "500000", "validation failed"))
		if !errors.Is(top, dbErr) {
			t.Fatal("expected Is to find plain error through wrap chain into Errors collection")
		}
	})
}
