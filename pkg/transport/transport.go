package transport

import (
	"context"

	"github.com/InTacht/xqua-go/pkg/ctx"
	"github.com/InTacht/xqua-go/pkg/logger"
)

// Transport manages the lifecycle of a network transport such as HTTP or gRPC.
type Transport interface {
	Name() string
	Serve(opts ServeOptions) error
	Shutdown(ctx context.Context) error
}

// ServeOptions configures transport startup behavior.
type ServeOptions struct {
	OnReady func()
}

type CreateTransportFunc[T ctx.Ctx] func(ctx T, logger *logger.Logger) Transport
