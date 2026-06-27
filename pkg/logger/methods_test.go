package logger_test

import (
	"context"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/logger"

	"go.uber.org/zap/zapcore"
)

func TestLogLevels(t *testing.T) {
	log, recorded := newObservedLogger(t, nil, zapcore.DebugLevel)
	ctx := logger.ContextWithRequestID(context.Background(), "req-1")

	log.Debug("debug", "message")
	log.DebugCtx(ctx, "debug", "ctx")
	if err := log.DebugWrap("debug", "wrap"); err != nil {
		t.Fatalf("DebugWrap should return nil, got %v", err)
	}
	if err := log.DebugWrapCtx(ctx, "debug", "wrap ctx"); err != nil {
		t.Fatalf("DebugWrapCtx should return nil, got %v", err)
	}

	log.Info("info", "message")
	log.InfoCtx(context.Background(), "no request id")
	log.InfoCtx(ctx, "info", "ctx")
	if err := log.InfoWrap("info", "wrap"); err != nil {
		t.Fatalf("InfoWrap should return nil, got %v", err)
	}
	if err := log.InfoWrapCtx(ctx, "info", "wrap ctx"); err != nil {
		t.Fatalf("InfoWrapCtx should return nil, got %v", err)
	}

	log.Warn("warn", "message")
	log.WarnCtx(ctx, "warn", "ctx")
	if err := log.WarnWrap("warn", "wrap"); err != nil {
		t.Fatalf("WarnWrap should return nil, got %v", err)
	}
	if err := log.WarnWrapCtx(ctx, "warn", "wrap ctx"); err != nil {
		t.Fatalf("WarnWrapCtx should return nil, got %v", err)
	}

	errVal := errors.NewPlain("boom")
	log.Error(errVal, "error", "message")
	log.ErrorCtx(ctx, errVal, "error", "ctx")
	if got := log.ErrorWrap(errVal, "error", "wrap"); got != errVal {
		t.Fatalf("ErrorWrap should return the error, got %v", got)
	}
	if got := log.ErrorWrapCtx(ctx, errVal, "error", "wrap ctx"); got != errVal {
		t.Fatalf("ErrorWrapCtx should return the error, got %v", got)
	}

	if n := len(recorded.AllUntimed()); n != 17 {
		t.Fatalf("expected 17 log entries, got %d", n)
	}

	ctxEntry := recorded.FilterMessage("info. ctx").AllUntimed()[0].ContextMap()
	if ctxEntry["request_id"] != "req-1" {
		t.Fatalf("expected request_id on context log, got %#v", ctxEntry)
	}
}
