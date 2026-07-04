package logger_test

import (
	"context"
	"testing"

	"github.com/InTacht/xqua-go/pkg/logger"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewDefaults(t *testing.T) {
	log := logger.New(nil)
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
	log.Close()
}

func TestNewWithNameAndID(t *testing.T) {
	log := logger.New(&logger.Config{Name: "orders", ID: "orders-1"})
	if log.Zap() == nil {
		t.Fatal("expected zap logger")
	}
	log.Close()
}

func TestNewDebugWithoutNameOrID(t *testing.T) {
	log := logger.New(&logger.Config{Debug: true})
	if log.Zap() == nil {
		t.Fatal("expected zap logger")
	}
	log.Close()
}

func TestFromZapBuildsWhenNil(t *testing.T) {
	log := logger.FromZap(&logger.Config{Name: "svc", ID: "svc-1"}, nil)
	if log.Zap() == nil {
		t.Fatal("expected built zap logger")
	}
}

func TestConfigDefaults(t *testing.T) {
	log, recorded := newObservedLogger(t, &logger.Config{}, zapcore.InfoLevel)
	ctx := logger.ContextWithRequestID(context.Background(), "req-1")
	log.InfoCtx(ctx, "hello")

	fields := recorded.AllUntimed()[0].ContextMap()
	if fields["request_id"] != "req-1" {
		t.Fatalf("expected default request_id field key, got %#v", fields)
	}
}

func TestCustomRequestIDFieldKey(t *testing.T) {
	log, recorded := newObservedLogger(t, &logger.Config{
		RequestIDFieldKey: "trace_id",
	}, zapcore.InfoLevel)

	ctx := logger.ContextWithRequestID(context.Background(), "trace-1")
	log.InfoCtx(ctx, "handled")

	fields := recorded.AllUntimed()[0].ContextMap()
	if fields["trace_id"] != "trace-1" {
		t.Fatalf("expected trace_id field, got %#v", fields)
	}
}

func TestContextWithRequestID(t *testing.T) {
	log := logger.New(nil)
	ctx := logger.ContextWithRequestID(context.Background(), "req-123")

	id, ok := log.RequestID(ctx)
	if !ok {
		t.Fatal("expected request ID in context")
	}
	if id != "req-123" {
		t.Fatalf("expected req-123, got %q", id)
	}
}

func TestRequestIDEdgeCases(t *testing.T) {
	log := logger.New(nil)

	if id, ok := log.RequestID(context.TODO()); ok || id != "" {
		t.Fatal("expected false for nil context")
	}

	ctx := context.WithValue(context.Background(), struct{}{}, "wrong-type")
	if id, ok := log.RequestID(ctx); ok || id != "" {
		t.Fatal("expected false for wrong context value type")
	}

	ctx = logger.ContextWithRequestID(context.Background(), "")
	if id, ok := log.RequestID(ctx); ok || id != "" {
		t.Fatal("expected false for empty request ID")
	}
}

func TestDeriveLabel(t *testing.T) {
	root := logger.New(&logger.Config{Name: "orders", ID: "orders", Label: "server"})
	child := root.Derive("handler")
	same := child.Derive("")
	grandchild := child.Derive("GetUser")

	childLog, ok := child.(*logger.Logger)
	if !ok || childLog.Zap() == nil {
		t.Fatal("expected derived *Logger with zap core")
	}
	grandchildLog, ok := grandchild.(*logger.Logger)
	if !ok || grandchildLog.Zap() == nil {
		t.Fatal("expected grandchild *Logger with zap core")
	}
	if same == child {
		t.Fatal("Derive(\"\") should return a new logger")
	}
}

func TestJoinMessageEmpty(t *testing.T) {
	log := logger.New(nil)
	log.Debug()
	log.Close()
}

func TestZap(t *testing.T) {
	log := logger.FromZap(nil, zap.NewNop())
	if log.Zap() == nil {
		t.Fatal("expected underlying zap logger")
	}
}
