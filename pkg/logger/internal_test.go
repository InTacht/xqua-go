package logger

import (
	"fmt"
	"testing"
	"time"

	"github.com/InTacht/xqua-go/pkg/errors"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestJoinLabel(t *testing.T) {
	if got := joinLabel("server", "handler"); got != "server.handler" {
		t.Fatalf("got %q", got)
	}
	if got := joinLabel("server", ""); got != "server" {
		t.Fatalf("got %q", got)
	}
	if got := joinLabel("", "handler"); got != "handler" {
		t.Fatalf("got %q", got)
	}
}

func TestErrorObjectNil(t *testing.T) {
	var obj errorObject
	if err := obj.MarshalLogObject(zapcore.NewMapObjectEncoder()); err != nil {
		t.Fatalf("MarshalLogObject(nil): %v", err)
	}
}

func TestErrorObjectsSkipsNil(t *testing.T) {
	core, recorded := observer.New(zapcore.ErrorLevel)
	log := FromZap(nil, zap.New(core))

	errs := errors.Errors{
		nil,
		errors.New("validation", "422301", "required", "body.id"),
	}
	log.Error(errs, "validation failed")

	raw := recorded.AllUntimed()[0].ContextMap()["errors"].([]any)
	if len(raw) != 1 {
		t.Fatalf("expected 1 encoded error, got %d", len(raw))
	}
}

func TestErrorObjectsMarshalFailure(t *testing.T) {
	err := errorObjects{errors.New("validation", "422301", "fail")}.MarshalLogArray(failArrayEncoder{})
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

type failArrayEncoder struct{}

func (failArrayEncoder) AppendArray(zapcore.ArrayMarshaler) error   { return nil }
func (failArrayEncoder) AppendObject(zapcore.ObjectMarshaler) error { return fmt.Errorf("fail") }
func (failArrayEncoder) AppendReflected(any) error                  { return nil }
func (failArrayEncoder) AppendBool(bool)                            {}
func (failArrayEncoder) AppendByteString([]byte)                    {}
func (failArrayEncoder) AppendComplex128(complex128)                {}
func (failArrayEncoder) AppendComplex64(complex64)                  {}
func (failArrayEncoder) AppendDuration(time.Duration)               {}
func (failArrayEncoder) AppendFloat64(float64)                      {}
func (failArrayEncoder) AppendFloat32(float32)                      {}
func (failArrayEncoder) AppendInt(int)                              {}
func (failArrayEncoder) AppendInt64(int64)                          {}
func (failArrayEncoder) AppendInt32(int32)                          {}
func (failArrayEncoder) AppendInt16(int16)                          {}
func (failArrayEncoder) AppendInt8(int8)                            {}
func (failArrayEncoder) AppendString(string)                        {}
func (failArrayEncoder) AppendTime(time.Time)                       {}
func (failArrayEncoder) AppendUint(uint)                            {}
func (failArrayEncoder) AppendUint64(uint64)                        {}
func (failArrayEncoder) AppendUint32(uint32)                        {}
func (failArrayEncoder) AppendUint16(uint16)                        {}
func (failArrayEncoder) AppendUint8(uint8)                          {}
func (failArrayEncoder) AppendUintptr(uintptr)                      {}
