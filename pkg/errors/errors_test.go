package errors_test

import (
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
)

func TestError(t *testing.T) {
	t.Run("New variadic args", func(t *testing.T) {
		full := errors.New("validation", "422301", "required", "body.id")
		if full.Kind != "validation" || full.Code != "422301" || full.Message != "required" || full.Source != "body.id" {
			t.Fatalf("unexpected full error: %+v", full)
		}

		partial := errors.New("internal", "500001", "query failed")
		if partial.Source != "" {
			t.Fatalf("expected empty source, got %q", partial.Source)
		}

		minimal := errors.New("not_found", "404301")
		if minimal.Message != "" || minimal.Source != "" {
			t.Fatalf("expected empty message and source, got %+v", minimal)
		}

		empty := errors.New()
		if empty.Kind != "" || empty.Code != "" {
			t.Fatal("expected all fields empty")
		}

		truncated := errors.New("a", "b", "c", "d", "ignored")
		if truncated.Source != "d" {
			t.Fatalf("expected source d, got %q", truncated.Source)
		}
	})

	t.Run("NewPlain delegates to standard library", func(t *testing.T) {
		err := errors.NewPlain("connection reset by peer")
		if err.Error() != "connection reset by peer" {
			t.Fatalf("unexpected message: %q", err.Error())
		}
		if errors.Is(err, errors.NewPlain("connection reset by peer")) {
			t.Fatal("expected Is to match by identity, not message")
		}
	})

	t.Run("Error string format", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			err  *errors.Error
			want string
		}{
			{
				name: "full",
				err:  errors.New("validation", "422301", "required", "body.id"),
				want: "validation<422301>(body.id): required",
			},
			{
				name: "without source",
				err:  errors.New("internal", "500001", "query failed"),
				want: "internal<500001>: query failed",
			},
			{
				name: "code and message only",
				err:  errors.New("", "500000", "fallback"),
				want: "<500000>: fallback",
			},
			{
				name: "kind and code only",
				err:  errors.New("not_found", "404301"),
				want: "not_found<404301>",
			},
			{
				name: "empty",
				err:  errors.New(),
				want: "",
			},
			{
				name: "nil receiver",
				err:  nil,
				want: "",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var got string
				if tc.err != nil {
					got = tc.err.Error()
				} else {
					var nilErr *errors.Error
					got = nilErr.Error()
				}
				if got != tc.want {
					t.Fatalf("Error() = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("WithSource copies without mutating original", func(t *testing.T) {
		err := errors.New("not_found", "404301", "subscriber not found")
		withSource := err.WithSource("params.id")
		if withSource.Source != "params.id" {
			t.Fatalf("unexpected source: %s", withSource.Source)
		}
		if err.Source != "" {
			t.Fatal("original error should be unchanged")
		}
	})

	t.Run("WithMessage copies without changing code", func(t *testing.T) {
		err := errors.New("validation", "422301", "id is required", "body.id")
		withMessage := err.WithMessage("custom message")
		if withMessage.Message != "custom message" {
			t.Fatalf("unexpected message: %q", withMessage.Message)
		}
		if withMessage.Code != err.Code {
			t.Fatal("WithMessage should not change code")
		}
	})

	t.Run("nil receiver helpers", func(t *testing.T) {
		var err *errors.Error
		if err.Unwrap() != nil {
			t.Fatal("Unwrap on nil receiver should return nil")
		}
		if err.WithSource("x") != nil {
			t.Fatal("WithSource on nil receiver should return nil")
		}
		if err.WithMessage("x") != nil {
			t.Fatal("WithMessage on nil receiver should return nil")
		}
	})
}

func TestErrorsCollection(t *testing.T) {
	catalog := newTestCatalog()

	t.Run("Error joins members with newlines", func(t *testing.T) {
		errs := errors.Errors{catalog.errIDRequired, catalog.errEmailInvalid}
		want := "validation<422301>(body.id): id is required\nvalidation<422302>(body.email): email is invalid"
		if got := errs.Error(); got != want {
			t.Fatalf("Errors.Error() = %q, want %q", got, want)
		}
	})

	t.Run("empty collection Error returns empty string", func(t *testing.T) {
		var empty errors.Errors
		if empty.Error() != "" {
			t.Fatal("expected empty string for empty collection")
		}
	})

	t.Run("skips nil members in Error string", func(t *testing.T) {
		errs := errors.Errors{nil, catalog.errIDRequired}
		if got := errs.Error(); got != catalog.errIDRequired.Error() {
			t.Fatalf("Errors.Error() = %q, want %q", got, catalog.errIDRequired.Error())
		}
	})

	t.Run("Append skips nil entries", func(t *testing.T) {
		errs := errors.Errors{catalog.errIDRequired, catalog.errEmailInvalid}
		appended := errs.Append(nil, catalog.errUserNotFound)
		if len(appended) != 3 {
			t.Fatalf("expected 3 after append, got %d", len(appended))
		}
	})
}

func TestExtract(t *testing.T) {
	t.Run("AsErrors single catalog entry", func(t *testing.T) {
		err := errors.New("validation", "422301", "external_id is required", "body.external_id")
		got := errors.AsErrors(err)
		if len(got) != 1 || got[0].Code != "422301" {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("AsErrors collection", func(t *testing.T) {
		errs := errors.Errors{
			errors.New("validation", "422301", "external_id is required", "body.external_id"),
			errors.New("validation", "422302", "invalid email", "body.email"),
		}
		if len(errors.AsErrors(errs)) != 2 {
			t.Fatal("expected 2 errors")
		}
	})

	t.Run("AsErrors empty collection returns nil", func(t *testing.T) {
		var empty errors.Errors
		if errors.AsErrors(empty) != nil {
			t.Fatal("expected nil for empty collection")
		}
	})

	t.Run("AsErrors plain error returns nil", func(t *testing.T) {
		if errors.AsErrors(errors.NewPlain("plain error")) != nil {
			t.Fatal("expected nil for non-structured errors")
		}
	})

	t.Run("AsErrors nil returns nil", func(t *testing.T) {
		if errors.AsErrors(nil) != nil {
			t.Fatal("expected nil")
		}
	})

	t.Run("IsStructured", func(t *testing.T) {
		if !errors.IsStructured(errors.New("validation", "422301", "required")) {
			t.Fatal("expected catalog error to be structured")
		}
		if errors.IsStructured(errors.NewPlain("plain")) {
			t.Fatal("expected plain error to be unstructured")
		}
	})
}

// buildHybridTree constructs:
//
//	siblings
//	-> error1 -> error2 -> siblings { error21, error22 }
//	-> error3 -> siblings { error4, error5 }
//	-> error6
func buildHybridTree() errors.Errors {
	err21 := errors.New("validation", "422021", "error21")
	err22 := errors.New("validation", "422022", "error22")
	err2 := errors.New("internal", "500002", "error2")
	err1 := errors.New("internal", "500001", "error1")

	branch1 := errors.Wrap(
		errors.Wrap(errors.Errors{err21, err22}, err2),
		err1,
	).(*errors.Error)

	err4 := errors.New("validation", "422004", "error4")
	err5 := errors.New("validation", "422005", "error5")
	err3 := errors.New("internal", "500003", "error3")

	branch2 := errors.Wrap(
		errors.Errors{err4, err5},
		err3,
	).(*errors.Error)

	err6 := errors.New("not_found", "404006", "error6")

	return errors.Errors{branch1, branch2, err6}
}

func TestHybridTree(t *testing.T) {
	hybrid := buildHybridTree()

	t.Run("AsErrors returns top-level branch heads only", func(t *testing.T) {
		got := errors.AsErrors(hybrid)
		if len(got) != 3 {
			t.Fatalf("expected 3 top-level entries, got %d", len(got))
		}
		if got[0].Code != "500001" || got[1].Code != "500003" || got[2].Code != "404006" {
			t.Fatalf("unexpected top-level codes: %v", codes(got))
		}
	})

	t.Run("stdlib Is finds nested catalog entries", func(t *testing.T) {
		targets := []*errors.Error{
			errors.New("internal", "500001", "error1"),
			errors.New("internal", "500002", "error2"),
			errors.New("validation", "422021", "error21"),
			errors.New("validation", "422022", "error22"),
			errors.New("internal", "500003", "error3"),
			errors.New("validation", "422004", "error4"),
			errors.New("validation", "422005", "error5"),
			errors.New("not_found", "404006", "error6"),
		}
		for _, target := range targets {
			if !errors.Is(hybrid, target) {
				t.Fatalf("expected Is to find %s/%s", target.Kind, target.Code)
			}
		}
	})

	t.Run("Cause from branch head reaches nested siblings", func(t *testing.T) {
		branch1 := hybrid[0]
		if errors.Cause(branch1).(*errors.Error).Code != "500002" {
			t.Fatalf("expected error2 as immediate cause, got %v", errors.Cause(branch1))
		}
		nested, ok := errors.Cause(errors.Cause(branch1)).(errors.Errors)
		if !ok || len(nested) != 2 {
			t.Fatalf("expected nested siblings, got %T", errors.Cause(errors.Cause(branch1)))
		}
	})
}
