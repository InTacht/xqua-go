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

var demoCatalog = errors.NewCatalog("logging-demo")

var (
	errQueryFailed = demoCatalog.Define(errors.Def{
		Code: "1001", Message: "query failed",
	})
	errFetchFailed = demoCatalog.Define(errors.Def{
		Code: "1002", Message: "fetch user failed",
	})
	errIDRequired = demoCatalog.Define(errors.Def{
		Code: "2001", Message: "id is required", Source: "body.id",
	})
	errEmailInvalid = demoCatalog.Define(errors.Def{
		Code: "2002", Message: "email is invalid", Source: "body.email",
	})
	errValidationFailed = demoCatalog.Define(errors.Def{
		Code: "1000", Message: "validation failed",
	})
)

func main() {
	// Root logger only: New + Close. Derive children for labels; never Close them.
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
	return errors.Wrap(errs, errValidationFailed)
}

func hybridTree() errors.Errors {
	c := errors.NewCatalog("hybrid-demo")
	err21 := c.Define(errors.Def{Code: "021", Message: "error21"})
	err22 := c.Define(errors.Def{Code: "022", Message: "error22"})
	err2 := c.Define(errors.Def{Code: "002", Message: "error2"})
	err1 := c.Define(errors.Def{Code: "001", Message: "error1"})

	branch1 := errors.Wrap(
		errors.Wrap(errors.Errors{err21, err22}, err2),
		err1,
	).(*errors.Error)

	err4 := c.Define(errors.Def{Code: "004", Message: "error4"})
	err5 := c.Define(errors.Def{Code: "005", Message: "error5"})
	err3 := c.Define(errors.Def{Code: "003", Message: "error3"})

	branch2 := errors.Wrap(
		errors.Errors{err4, err5},
		err3,
	).(*errors.Error)

	err6 := c.Define(errors.Def{Code: "006", Message: "error6"})

	return errors.Errors{branch1, branch2, err6}
}
