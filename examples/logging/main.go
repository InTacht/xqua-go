// Run: go run ./examples/logging
//
// Each log line records one layer only: an "errors" array for structured
// failures (with cause nested in each object when wrapped), or an "error"
// string for plain failures. Filter by request_id and read logs in timestamp
// order to rebuild the full trace across call layers.
package main

import (
	"context"
	"fmt"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/logger"
)

var (
	errQueryFailed  = errors.New("internal", "500001", "query failed")
	errFetchFailed  = errors.New("internal", "500002", "fetch user failed")
	errIDRequired   = errors.New("validation", "422301", "id is required", "body.id")
	errEmailInvalid = errors.New("validation", "422302", "email is invalid", "body.email")
)

func main() {
	log := logger.New(&logger.Config{
		Name:  "logging-demo",
		ID:    "logging-demo",
		Label: "demo",
		Debug: true,
	})
	defer log.Close()

	ctx := logger.ContextWithRequestID(context.Background(), "req-demo-1")

	scenario := func(title string, err error) {
		fmt.Println()
		fmt.Println("---", title, "---")
		log.ErrorCtx(ctx, err, title)
	}

	scenario("1. single wrap: structured error over plain driver error", wrapSingle())
	scenario("2. one layer of a vertical chain (service view)", wrapVertical())
	logTraceAcrossLayers(log, ctx)
	scenario("4. horizontal collection: validation siblings", validationCollection())
	scenario("5. outer wrap over Errors collection", wrapOverCollection())
	scenario("6. plain unstructured error", errors.NewPlain("something broke"))
	scenario("7. hybrid tree: siblings with nested wraps", hybridTree())
}

func wrapSingle() error {
	dbErr := errors.NewPlain("connection reset by peer")
	return errors.Wrap(dbErr, errQueryFailed)
}

func wrapVertical() error {
	dbErr := errors.NewPlain("connection reset by peer")
	repoErr := errors.Wrap(dbErr, errQueryFailed)
	return errors.Wrap(repoErr, errFetchFailed)
}

func logTraceAcrossLayers(log *logger.Logger, ctx context.Context) {
	fmt.Println()
	fmt.Println("--- 3. trace across repo then service (same request_id) ---")

	dbErr := errors.NewPlain("connection reset by peer")
	repoErr := errors.Wrap(dbErr, errQueryFailed)
	serviceErr := errors.Wrap(repoErr, errFetchFailed)

	log.Derive("repository").ErrorCtx(ctx, repoErr, "repository: query failed")
	log.Derive("service").ErrorCtx(ctx, serviceErr, "service: fetch user failed")
}

func validationCollection() error {
	return errors.Errors{
		errIDRequired,
		errEmailInvalid,
	}
}

func wrapOverCollection() error {
	errs := errors.Errors{
		errIDRequired,
		errEmailInvalid,
	}
	return errors.Wrap(errs, errors.New("internal", "500000", "validation failed"))
}

func hybridTree() errors.Errors {
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
