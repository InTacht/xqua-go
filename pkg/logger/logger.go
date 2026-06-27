package logger

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	DefaultRequestIDHeaderKey = "X-Request-ID"
	DefaultRequestIDFieldKey  = "request_id"
	defaultLabel              = "logger"
)

type contextKey struct{}

var requestIDContextKey = contextKey{}

// Config configures a Logger instance.
type Config struct {
	Name string
	ID   string

	Label string
	Debug bool

	// RequestIDHeaderKey is the HTTP header used to propagate request IDs.
	RequestIDHeaderKey string
	// RequestIDFieldKey is the structured log field name for request IDs.
	RequestIDFieldKey string
}

// Logger wraps zap with XQUA logging conventions.
type Logger struct {
	config *Config
	logger *zap.Logger
}

// FromZap wraps an existing zap logger with XQUA config metadata.
func FromZap(config *Config, zl *zap.Logger) *Logger {
	cfg := normalizeConfig(config)
	if zl == nil {
		zl = buildZapLogger(cfg)
	}
	return &Logger{config: cfg, logger: zl}
}

// New creates a Logger from config.
func New(config *Config) *Logger {
	cfg := normalizeConfig(config)
	zl := buildZapLogger(cfg)
	zl.Debug("logger initialized")

	return &Logger{config: cfg, logger: zl}
}

// Derive returns a child logger with label extended by the given segment.
func (l *Logger) Derive(label string) *Logger {
	cfg := *l.config
	cfg.Label = joinLabel(l.config.Label, label)

	child := l.logger.With(
		zap.String("name", cfg.Name),
		zap.String("id", cfg.ID),
		zap.String("label", cfg.Label),
	)

	return &Logger{config: &cfg, logger: child}
}

// Zap exposes the underlying zap logger for middleware and third-party integrations.
func (l *Logger) Zap() *zap.Logger {
	return l.logger
}

// ContextWithRequestID attaches a request ID to ctx for structured logging.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

// RequestID returns the request ID stored in ctx, if present.
func (l *Logger) RequestID(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}

	id, ok := ctx.Value(requestIDContextKey).(string)
	if !ok || id == "" {
		return "", false
	}

	return id, true
}

// Close flushes buffered log entries. Sync errors on stdout/stderr are ignored.
func (l *Logger) Close() {
	l.logger.Debug("logger closed")
	_ = l.logger.Sync()
}

// DeInit is deprecated; use Close instead.
func (l *Logger) DeInit() {
	l.Close()
}

func normalizeConfig(config *Config) *Config {
	cfg := Config{}
	if config != nil {
		cfg = *config
	}

	if cfg.Label == "" {
		cfg.Label = defaultLabel
	}
	if cfg.RequestIDHeaderKey == "" {
		cfg.RequestIDHeaderKey = DefaultRequestIDHeaderKey
	}
	if cfg.RequestIDFieldKey == "" {
		cfg.RequestIDFieldKey = DefaultRequestIDFieldKey
	}

	return &cfg
}

func buildZapLogger(cfg *Config) *zap.Logger {
	zapCfg := zap.NewProductionConfig()
	zapCfg.EncoderConfig.TimeKey = "timestamp"
	zapCfg.EncoderConfig.MessageKey = "message"
	zapCfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)

	if cfg.Debug {
		zapCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}

	logger, err := zapCfg.Build()
	if err != nil {
		panic(fmt.Sprintf("logger: failed to build zap logger: %v", err))
	}

	fields := make([]zap.Field, 0, 3)
	if cfg.Name != "" {
		fields = append(fields, zap.String("name", cfg.Name))
	}
	if cfg.ID != "" {
		fields = append(fields, zap.String("id", cfg.ID))
	}
	fields = append(fields, zap.String("label", cfg.Label))

	return logger.With(fields...)
}

func joinLabel(parent, child string) string {
	switch {
	case child == "":
		return parent
	case parent == "":
		return child
	default:
		return parent + "." + child
	}
}
