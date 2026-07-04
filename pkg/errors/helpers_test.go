package errors_test

import "github.com/InTacht/xqua-go/pkg/errors"

// Category-named catalogs so entries default to their expected semantic Kind.
var (
	notFoundCat   = errors.NewCatalog("not_found")
	internalCat   = errors.NewCatalog("internal")
	validationCat = errors.NewCatalog("validation")

	testEntries = struct {
		errUserNotFound    *errors.Error
		errQueryFailed     *errors.Error
		errFetchUserFailed *errors.Error
		errIDRequired      *errors.Error
		errEmailInvalid    *errors.Error
		errFallback        *errors.Error
		errValidationFail  *errors.Error
		errWrongCode       *errors.Error
		errOrderNotFound   *errors.Error
	}{
		errUserNotFound: notFoundCat.Define(errors.Def{
			Code: "404301", Message: "user not found", Source: "params.id",
		}),
		errQueryFailed: internalCat.Define(errors.Def{
			Code: "500001", Message: "query failed",
		}),
		errFetchUserFailed: internalCat.Define(errors.Def{
			Code: "500002", Message: "fetch user failed",
		}),
		errIDRequired: validationCat.Define(errors.Def{
			Code: "422301", Message: "id is required", Source: "body.id",
		}),
		errEmailInvalid: validationCat.Define(errors.Def{
			Code: "422302", Message: "email is invalid", Source: "body.email",
		}),
		errFallback: internalCat.Define(errors.Def{
			Code: "500000", Message: "fallback message",
		}),
		errValidationFail: internalCat.Define(errors.Def{
			Code: "500010", Message: "validation failed",
		}),
		errWrongCode: internalCat.Define(errors.Def{
			Code: "500999", Message: "wrong code",
		}),
		errOrderNotFound: notFoundCat.Define(errors.Def{
			Code: "404302", Message: "order not found", Source: "params.id",
		}),
	}
)

func newTestCatalog() struct {
	errUserNotFound    *errors.Error
	errQueryFailed     *errors.Error
	errFetchUserFailed *errors.Error
	errIDRequired      *errors.Error
	errEmailInvalid    *errors.Error
	errFallback        *errors.Error
	errValidationFail  *errors.Error
	errWrongCode       *errors.Error
	errOrderNotFound   *errors.Error
} {
	return testEntries
}

func codes(errs []*errors.Error) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Code
	}
	return out
}

type hybridRefs struct {
	err1, err2, err21, err22, err3, err4, err5, err6 *errors.Error
}

// buildHybridTree constructs a hybrid tree and returns the entry references so
// tests can match against the very entries the tree was built from (same
// catalog, so Kind+Code identities line up).
func buildHybridTree() (errors.Errors, hybridRefs) {
	c := errors.NewCatalog("hybrid")
	refs := hybridEntries(c)

	branch1 := errors.Wrap(
		errors.Wrap(errors.Errors{refs.err21, refs.err22}, refs.err2),
		refs.err1,
	).(*errors.Error)

	branch2 := errors.Wrap(
		errors.Errors{refs.err4, refs.err5},
		refs.err3,
	).(*errors.Error)

	return errors.Errors{branch1, branch2, refs.err6}, refs
}

func hybridEntries(c *errors.Catalog) hybridRefs {
	return hybridRefs{
		err21: c.Define(errors.Def{Code: "422021", Message: "error21"}),
		err22: c.Define(errors.Def{Code: "422022", Message: "error22"}),
		err2:  c.Define(errors.Def{Code: "500002", Message: "error2"}),
		err1:  c.Define(errors.Def{Code: "500001", Message: "error1"}),
		err4:  c.Define(errors.Def{Code: "422004", Message: "error4"}),
		err5:  c.Define(errors.Def{Code: "422005", Message: "error5"}),
		err3:  c.Define(errors.Def{Code: "500003", Message: "error3"}),
		err6:  c.Define(errors.Def{Code: "404006", Message: "error6"}),
	}
}
