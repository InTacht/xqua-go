package logger

import (
	"context"
	"strings"

	"go.uber.org/zap"
)

func (l *Logger) Debug(msg ...string) {
	l.write(l.caller().Debug, msg...)
}

func (l *Logger) DebugCtx(ctx context.Context, msg ...string) {
	l.writeCtx(ctx, l.caller().Debug, msg...)
}

func (l *Logger) DebugWrap(msg ...string) error {
	l.write(l.caller().Debug, msg...)
	return nil
}

func (l *Logger) DebugWrapCtx(ctx context.Context, msg ...string) error {
	l.writeCtx(ctx, l.caller().Debug, msg...)
	return nil
}

func (l *Logger) Info(msg ...string) {
	l.write(l.caller().Info, msg...)
}

func (l *Logger) InfoCtx(ctx context.Context, msg ...string) {
	l.writeCtx(ctx, l.caller().Info, msg...)
}

func (l *Logger) InfoWrap(msg ...string) error {
	l.write(l.caller().Info, msg...)
	return nil
}

func (l *Logger) InfoWrapCtx(ctx context.Context, msg ...string) error {
	l.writeCtx(ctx, l.caller().Info, msg...)
	return nil
}

func (l *Logger) Warn(msg ...string) {
	l.write(l.caller().Warn, msg...)
}

func (l *Logger) WarnCtx(ctx context.Context, msg ...string) {
	l.writeCtx(ctx, l.caller().Warn, msg...)
}

func (l *Logger) WarnWrap(msg ...string) error {
	l.write(l.caller().Warn, msg...)
	return nil
}

func (l *Logger) WarnWrapCtx(ctx context.Context, msg ...string) error {
	l.writeCtx(ctx, l.caller().Warn, msg...)
	return nil
}

func (l *Logger) Error(err error, msg ...string) {
	l.writeError(l.caller().Error, err, msg...)
}

func (l *Logger) ErrorCtx(ctx context.Context, err error, msg ...string) {
	l.writeErrorCtx(ctx, l.caller().Error, err, msg...)
}

func (l *Logger) ErrorWrap(err error, msg ...string) error {
	l.writeError(l.caller().Error, err, msg...)
	return err
}

func (l *Logger) ErrorWrapCtx(ctx context.Context, err error, msg ...string) error {
	l.writeErrorCtx(ctx, l.caller().Error, err, msg...)
	return err
}

func (l *Logger) caller() *zap.Logger {
	return l.logger.WithOptions(zap.AddCallerSkip(1))
}

func (l *Logger) write(logFn func(string, ...zap.Field), msg ...string) {
	logFn(joinMessage(msg))
}

func (l *Logger) writeCtx(ctx context.Context, logFn func(string, ...zap.Field), msg ...string) {
	logFn(joinMessage(msg), l.requestIDField(ctx)...)
}

func (l *Logger) writeError(logFn func(string, ...zap.Field), err error, msg ...string) {
	fields := l.errorFields(err)
	logFn(joinMessage(msg), fields...)
}

func (l *Logger) writeErrorCtx(ctx context.Context, logFn func(string, ...zap.Field), err error, msg ...string) {
	fields := append(l.requestIDField(ctx), l.errorFields(err)...)
	logFn(joinMessage(msg), fields...)
}

func (l *Logger) requestIDField(ctx context.Context) []zap.Field {
	if id, ok := l.RequestID(ctx); ok {
		return []zap.Field{zap.String(l.config.RequestIDFieldKey, id)}
	}
	return nil
}

func joinMessage(msg []string) string {
	return strings.Join(msg, ". ")
}
