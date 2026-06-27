package errors_test

import "github.com/InTacht/xqua-go/pkg/errors"

type testCatalog struct {
	errUserNotFound    *errors.Error
	errQueryFailed     *errors.Error
	errFetchUserFailed *errors.Error
	errIDRequired      *errors.Error
	errEmailInvalid    *errors.Error
}

func newTestCatalog() testCatalog {
	return testCatalog{
		errUserNotFound:    errors.New("not_found", "404301", "user not found", "params.id"),
		errQueryFailed:     errors.New("internal", "500001", "query failed"),
		errFetchUserFailed: errors.New("internal", "500002", "fetch user failed"),
		errIDRequired:      errors.New("validation", "422301", "id is required", "body.id"),
		errEmailInvalid:    errors.New("validation", "422302", "email is invalid", "body.email"),
	}
}

func codes(errs []*errors.Error) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Code
	}
	return out
}
