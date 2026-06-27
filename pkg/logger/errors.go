package logger

import (
	"github.com/InTacht/xqua-go/pkg/errors"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type errorObject struct {
	err *errors.Error
}

func (o errorObject) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	if o.err == nil {
		return nil
	}

	enc.AddString("kind", o.err.Kind)
	enc.AddString("code", o.err.Code)
	enc.AddString("message", o.err.Message)

	if o.err.Source != "" {
		enc.AddString("source", o.err.Source)
	}

	if cause := errors.Cause(o.err); cause != nil {
		enc.AddString("cause", cause.Error())
	}

	return nil
}

type errorObjects []*errors.Error

func (e errorObjects) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	for _, err := range e {
		if err == nil {
			continue
		}
		if err := enc.AppendObject(errorObject{err: err}); err != nil {
			return err
		}
	}
	return nil
}

// errorFields logs one layer at a time so a request trace is rebuilt from
// ordered log lines (e.g. by request_id), not a flattened chain per entry.
//
//   - Structured *errors.Error or Errors: "errors" array of objects (kind,
//     code, message, source, cause when wrapped).
//   - Plain error: "error" string.
func (l *Logger) errorFields(err error) []zap.Field {
	if err == nil {
		return nil
	}

	// If err can be converted to a canonical error collection
	// log it as an array of error objects.
	if es := errors.AsErrors(err); len(es) > 0 {
		return []zap.Field{zap.Array("errors", errorObjects(es))}
	}

	// Plain error, log it as a string.
	return []zap.Field{zap.Error(err)}
}
