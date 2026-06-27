package logger_test

import (
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"

	"go.uber.org/zap/zapcore"
)

func TestErrorFields(t *testing.T) {
	t.Run("nil error omits error fields", func(t *testing.T) {
		log, recorded := newObservedLogger(t, nil, zapcore.ErrorLevel)
		log.Error(nil, "message only")

		fields := recorded.AllUntimed()[0].ContextMap()
		if _, ok := fields["error"]; ok {
			t.Fatalf("expected no error field, got %#v", fields)
		}
		if _, ok := fields["errors"]; ok {
			t.Fatalf("expected no errors field, got %#v", fields)
		}
	})

	t.Run("plain error logs string field", func(t *testing.T) {
		log, recorded := newObservedLogger(t, nil, zapcore.ErrorLevel)
		log.Error(errors.NewPlain("plain failure"), "operation failed")

		fields := recorded.AllUntimed()[0].ContextMap()
		if fields["error"] != "plain failure" {
			t.Fatalf("expected plain error string, got %#v", fields["error"])
		}
		if _, ok := fields["errors"]; ok {
			t.Fatal("expected no errors array for plain error")
		}
	})

	t.Run("Errors collection logs errors array", func(t *testing.T) {
		log, recorded := newObservedLogger(t, nil, zapcore.ErrorLevel)
		errs := errors.Errors{
			errors.New("validation", "422301", "id required", "body.id"),
			errors.New("validation", "422302", "email invalid", "body.email"),
		}
		log.Error(errs, "validation failed")

		items := errorsFieldObjects(t, recorded.AllUntimed()[0].ContextMap())
		if len(items) != 2 {
			t.Fatalf("expected 2 error objects, got %d", len(items))
		}
	})

	t.Run("single canonical error logs one-item errors array", func(t *testing.T) {
		log, recorded := newObservedLogger(t, nil, zapcore.ErrorLevel)
		err := errors.New("validation", "422301", "required", "body.id")
		log.Error(err, "validation failed")

		errObj := firstErrorObject(t, recorded.AllUntimed()[0].ContextMap())
		if errObj["source"] != "body.id" {
			t.Fatalf("expected source on error object, got %#v", errObj)
		}
		if _, ok := errObj["cause"]; ok {
			t.Fatal("expected no cause when error is not wrapped")
		}
	})

	t.Run("wrapped error logs errors array with nested cause", func(t *testing.T) {
		log, recorded := newObservedLogger(t, nil, zapcore.ErrorLevel)
		cause := errors.NewPlain("connection reset")
		err := errors.Wrap(cause, errors.New("internal", "500001", "database unavailable"))
		log.Error(err, "query failed")

		errObj := firstErrorObject(t, recorded.AllUntimed()[0].ContextMap())
		if errObj["kind"] != "internal" || errObj["code"] != "500001" {
			t.Fatalf("unexpected error object: %#v", errObj)
		}
		if errObj["cause"] != "connection reset" {
			t.Fatalf("expected cause inside error object, got %#v", errObj["cause"])
		}
		assertNoTopLevelCause(t, recorded.AllUntimed()[0].ContextMap())
	})

	t.Run("one layer per wrap logs outer error and immediate cause only", func(t *testing.T) {
		log, recorded := newObservedLogger(t, nil, zapcore.ErrorLevel)
		inner := errors.New("internal", "500001", "query failed")
		middle := errors.Wrap(errors.NewPlain("connection reset"), inner)
		err := errors.Wrap(middle, errors.New("internal", "500002", "fetch user failed"))
		log.Error(err, "request failed")

		errObj := firstErrorObject(t, recorded.AllUntimed()[0].ContextMap())
		if errObj["code"] != "500002" {
			t.Fatalf("expected outermost code only, got %#v", errObj)
		}
		if errObj["cause"] != middle.Error() {
			t.Fatalf("expected immediate cause %q, got %#v", middle.Error(), errObj["cause"])
		}
		assertNoTopLevelCause(t, recorded.AllUntimed()[0].ContextMap())
	})

	t.Run("wrap over Errors collection", func(t *testing.T) {
		log, recorded := newObservedLogger(t, nil, zapcore.ErrorLevel)
		errs := errors.Errors{
			errors.New("validation", "422301", "id required", "body.id"),
			errors.New("validation", "422302", "email invalid", "body.email"),
		}
		top := errors.Wrap(errs, errors.New("internal", "500000", "validation failed"))
		log.Error(top, "handler validation failed")

		errObj := firstErrorObject(t, recorded.AllUntimed()[0].ContextMap())
		if errObj["code"] != "500000" {
			t.Fatalf("expected outer error code, got %#v", errObj)
		}
		if errObj["cause"] != errs.Error() {
			t.Fatalf("expected Errors collection as cause, got %#v", errObj["cause"])
		}
		assertNoTopLevelCause(t, recorded.AllUntimed()[0].ContextMap())
	})

	t.Run("wrapped member in Errors collection includes cause on object", func(t *testing.T) {
		log, recorded := newObservedLogger(t, nil, zapcore.ErrorLevel)
		dbErr := errors.NewPlain("connection reset")
		errs := errors.Errors{
			errors.Wrap(dbErr, errors.New("internal", "500001", "query failed")).(*errors.Error),
			errors.New("validation", "422301", "id required", "body.id"),
		}
		log.Error(errs, "failures")

		items := errorsFieldObjects(t, recorded.AllUntimed()[0].ContextMap())
		wrapped := items[0].(map[string]any)
		plain := items[1].(map[string]any)

		if wrapped["cause"] != dbErr.Error() {
			t.Fatalf("expected cause on wrapped member, got %#v", wrapped)
		}
		if _, ok := plain["cause"]; ok {
			t.Fatal("expected no cause on unwrapped member")
		}
	})

	t.Run("trace across separate log lines", func(t *testing.T) {
		log, recorded := newObservedLogger(t, nil, zapcore.ErrorLevel)
		dbErr := errors.NewPlain("connection reset by peer")
		repoErr := errors.Wrap(dbErr, errors.New("internal", "500001", "query failed"))
		serviceErr := errors.Wrap(repoErr, errors.New("internal", "500002", "fetch user failed"))

		log.Error(repoErr, "repository query failed")
		log.Error(serviceErr, "service fetch failed")

		entries := recorded.AllUntimed()
		if len(entries) != 2 {
			t.Fatalf("expected 2 log entries, got %d", len(entries))
		}

		repoObj := firstErrorObject(t, entries[0].ContextMap())
		if repoObj["code"] != "500001" || repoObj["cause"] != dbErr.Error() {
			t.Fatalf("unexpected repo layer: %#v", repoObj)
		}

		serviceObj := firstErrorObject(t, entries[1].ContextMap())
		if serviceObj["code"] != "500002" || serviceObj["cause"] != repoErr.Error() {
			t.Fatalf("unexpected service layer: %#v", serviceObj)
		}
	})
}

func errorsFieldObjects(t *testing.T, fields map[string]any) []any {
	t.Helper()
	raw, ok := fields["errors"].([]any)
	if !ok {
		t.Fatalf("expected errors array field, got %#v", fields["errors"])
	}
	return raw
}

func firstErrorObject(t *testing.T, fields map[string]any) map[string]any {
	t.Helper()
	items := errorsFieldObjects(t, fields)
	if len(items) != 1 {
		t.Fatalf("expected 1 error object, got %d", len(items))
	}
	errObj, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map error object, got %#v", items[0])
	}
	return errObj
}

func assertNoTopLevelCause(t *testing.T, fields map[string]any) {
	t.Helper()
	if _, ok := fields["cause"]; ok {
		t.Fatal("expected cause nested in error object, not top-level")
	}
}
