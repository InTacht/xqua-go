package errors_test

import (
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
)

func TestCatalogDefine(t *testing.T) {
	c := errors.NewCatalog("define")

	t.Run("kind defaults to catalog name", func(t *testing.T) {
		entry := c.Define(errors.Def{
			Code: "422301", Message: "required", Source: "body.id",
		})
		if entry.Kind != "define" || entry.Code != "422301" {
			t.Fatalf("unexpected entry: %+v", entry)
		}
		if entry.Message != "required" || entry.Source != "body.id" {
			t.Fatalf("unexpected message/source: %+v", entry)
		}
	})

	t.Run("explicit semantic kind", func(t *testing.T) {
		entry := c.Define(errors.Def{
			Kind: "validation", Code: "422302", Message: "invalid",
		})
		if entry.Kind != "validation" {
			t.Fatalf("expected explicit kind, got %q", entry.Kind)
		}
	})

	t.Run("code may be any non-empty identifier", func(t *testing.T) {
		entry := c.Define(errors.Def{Code: "USER_MISSING", Message: "missing"})
		if entry.Code != "USER_MISSING" {
			t.Fatalf("expected freeform code, got %q", entry.Code)
		}
	})

	t.Run("empty code panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for empty code")
			}
		}()
		c.Define(errors.Def{Message: "no code"})
	})

	t.Run("duplicate code panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for duplicate code")
			}
		}()
		c.Define(errors.Def{Code: "422301", Message: "duplicate"})
	})
}

func TestNewCatalog(t *testing.T) {
	t.Run("empty name panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for empty name")
			}
		}()
		errors.NewCatalog("   ")
	})

	t.Run("names may repeat without identity collisions", func(t *testing.T) {
		a := errors.NewCatalog("shared-name")
		b := errors.NewCatalog("shared-name")
		errA := a.Define(errors.Def{Code: "10001", Message: "from a"})
		errB := b.Define(errors.Def{Code: "10001", Message: "from b"})

		if errors.Is(errA, errB) || errors.Is(errB, errA) {
			t.Fatal("entries from different catalogs must never match, even with equal kind and code")
		}
		if !errors.Is(errA, errA) {
			t.Fatal("entry must match itself")
		}
	})
}

func TestPointerIdentity(t *testing.T) {
	catalog := newTestCatalog()

	t.Run("clones match their template entry", func(t *testing.T) {
		clone := catalog.errUserNotFound.WithMessage("custom message").WithSource("query.id")
		if !errors.Is(clone, catalog.errUserNotFound) {
			t.Fatal("expected clone to match its catalog entry")
		}
		if !errors.Is(catalog.errUserNotFound, clone) {
			t.Fatal("expected match to be symmetric")
		}
	})

	t.Run("manually constructed errors never match", func(t *testing.T) {
		impostor := &errors.Error{
			Kind: catalog.errUserNotFound.Kind, Code: catalog.errUserNotFound.Code,
		}
		if errors.Is(impostor, catalog.errUserNotFound) {
			t.Fatal("kind/code strings alone must not grant identity")
		}
	})
}

func TestIsKind(t *testing.T) {
	catalog := newTestCatalog()

	t.Run("matches top-level kind", func(t *testing.T) {
		if !errors.IsKind(catalog.errIDRequired, "validation") {
			t.Fatal("expected kind match")
		}
		if errors.IsKind(catalog.errIDRequired, "not_found") {
			t.Fatal("expected kind mismatch")
		}
	})

	t.Run("matches shared kinds across catalogs", func(t *testing.T) {
		other := errors.NewCatalog("other").Define(errors.Def{
			Kind: "validation", Code: "1", Message: "other validation",
		})
		if !errors.IsKind(other, "validation") {
			t.Fatal("expected shared kind to match across catalogs")
		}
	})

	t.Run("walks wrap chains and collections", func(t *testing.T) {
		wrapped := errors.Wrap(catalog.errUserNotFound, catalog.errFetchUserFailed)
		if !errors.IsKind(wrapped, "not_found") {
			t.Fatal("expected kind found in wrap chain")
		}

		errs := errors.Errors{catalog.errQueryFailed, catalog.errIDRequired}
		top := errors.Wrap(errs, catalog.errValidationFail)
		if !errors.IsKind(top, "validation") {
			t.Fatal("expected kind found inside collection behind a wrap")
		}
	})

	t.Run("plain, nil, and empty kind", func(t *testing.T) {
		if errors.IsKind(errors.NewPlain("plain"), "validation") {
			t.Fatal("plain errors have no kind")
		}
		if errors.IsKind(nil, "validation") {
			t.Fatal("nil has no kind")
		}
		if errors.IsKind(catalog.errIDRequired, "") {
			t.Fatal("empty kind must not match")
		}
	})
}

func TestCatalogLookupAndEntries(t *testing.T) {
	c := errors.NewCatalog("lookup")
	a := c.Define(errors.Def{Code: "2", Message: "second"})
	b := c.Define(errors.Def{Code: "1", Message: "first"})

	t.Run("Lookup decodes code to entry", func(t *testing.T) {
		got, ok := c.Lookup("2")
		if !ok || got != a {
			t.Fatalf("expected entry a, got %v (ok=%v)", got, ok)
		}
		if _, ok := c.Lookup("missing"); ok {
			t.Fatal("expected miss for unknown code")
		}
	})

	t.Run("Entries sorted by code", func(t *testing.T) {
		entries := c.Entries()
		if len(entries) != 2 || entries[0] != b || entries[1] != a {
			t.Fatalf("unexpected entries order: %v", codes(entries))
		}
	})

	t.Run("Contains matches entries and clones only", func(t *testing.T) {
		if !c.Contains(a) || !c.Contains(a.WithMessage("clone")) {
			t.Fatal("expected entry and clone to be contained")
		}
		foreign := errors.NewCatalog("foreign").Define(errors.Def{Code: "2", Message: "impostor"})
		if c.Contains(foreign) {
			t.Fatal("foreign entry with same code must not be contained")
		}
		if c.Contains(nil) {
			t.Fatal("nil must not be contained")
		}
	})
}

func TestNewPlain(t *testing.T) {
	err := errors.NewPlain("connection reset by peer")
	if err.Error() != "connection reset by peer" {
		t.Fatalf("unexpected message: %q", err.Error())
	}
	if errors.Is(err, errors.NewPlain("connection reset by peer")) {
		t.Fatal("expected Is to match by identity, not message")
	}
}

func TestErrorStringFormat(t *testing.T) {
	catalog := newTestCatalog()

	for _, tc := range []struct {
		name string
		err  *errors.Error
		want string
	}{
		{
			name: "full",
			err:  catalog.errIDRequired,
			want: "validation<422301>(body.id): id is required",
		},
		{
			name: "without source",
			err:  catalog.errQueryFailed,
			want: "internal<500001>: query failed",
		},
		{
			name: "fallback",
			err:  catalog.errFallback,
			want: "internal<500000>: fallback message",
		},
		{
			name: "kind and code only",
			err:  catalog.errUserNotFound.WithMessage(""),
			want: "not_found<404301>(params.id)",
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
}

func TestWithSourceAndMessage(t *testing.T) {
	catalog := newTestCatalog()

	t.Run("WithSource copies without mutating original", func(t *testing.T) {
		err := catalog.errUserNotFound
		withSource := err.WithSource("params.id")
		if withSource.Source != "params.id" {
			t.Fatalf("unexpected source: %s", withSource.Source)
		}
		if err.Source != "params.id" {
			t.Fatal("original should keep catalog source")
		}
	})

	t.Run("WithMessage copies without changing code", func(t *testing.T) {
		err := catalog.errIDRequired
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
	catalog := newTestCatalog()

	t.Run("AsErrors single catalog entry", func(t *testing.T) {
		err := catalog.errIDRequired
		got := errors.AsErrors(err)
		if len(got) != 1 || got[0].Code != "422301" {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("AsErrors collection", func(t *testing.T) {
		errs := errors.Errors{catalog.errIDRequired, catalog.errEmailInvalid}
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
		if !errors.IsStructured(catalog.errIDRequired) {
			t.Fatal("expected catalog error to be structured")
		}
		if errors.IsStructured(errors.NewPlain("plain")) {
			t.Fatal("expected plain error to be unstructured")
		}
	})
}

func TestHybridTree(t *testing.T) {
	hybrid, refs := buildHybridTree()

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
			refs.err1, refs.err2, refs.err21, refs.err22,
			refs.err3, refs.err4, refs.err5, refs.err6,
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
