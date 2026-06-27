package logger_test

import (
	"testing"

	"github.com/InTacht/xqua-go/pkg/logger"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func newObservedLogger(t *testing.T, cfg *logger.Config, level zapcore.Level) (*logger.Logger, *observer.ObservedLogs) {
	t.Helper()
	core, recorded := observer.New(level)
	return logger.FromZap(cfg, zap.New(core)), recorded
}
