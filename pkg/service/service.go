package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/InTacht/xqua-go/pkg/ctx"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/transport"
)

// LifecycleHook runs during service startup or shutdown.
type LifecycleHook func(ctx context.Context) error

// Service orchestrates lifecycle hooks and registered transports.
type Service[T ctx.Ctx] struct {
	cfg Config
	ctx T

	log        *logger.Logger
	ownsLogger bool

	transports []transport.Transport

	onStartup  []LifecycleHook
	onShutdown []LifecycleHook
}

// New creates a Service from explicit configuration.
func New[T ctx.Ctx](cfg Config, ctx T) *Service[T] {
	if err := cfg.validate(); err != nil {
		panic(err)
	}

	if err := ctx.Build(context.Background()); err != nil {
		panic(err)
	}

	log, ownsLogger := cfg.logger()

	return &Service[T]{
		cfg:        cfg,
		ctx:        ctx,
		log:        log,
		ownsLogger: ownsLogger,
	}
}

// Logger returns the service logger.
func (s *Service[T]) Logger() *logger.Logger {
	return s.log
}

// Transport registers a transport to be started when Run is called.
func (s *Service[T]) Transport(t transport.CreateTransportFunc[T]) *Service[T] {
	if t != nil {
		s.transports = append(s.transports, t(s.ctx, s.log))
	}
	return s
}

// OnStartup registers a hook that runs before transports start serving.
func (s *Service[T]) OnStartup(fn LifecycleHook) *Service[T] {
	s.onStartup = append(s.onStartup, fn)
	return s
}

// OnShutdown registers a hook that runs after a stop signal and before transport shutdown.
// Hooks run in reverse registration order.
func (s *Service[T]) OnShutdown(fn LifecycleHook) *Service[T] {
	s.onShutdown = append(s.onShutdown, fn)
	return s
}

// Run executes startup hooks, serves all registered transports, and performs a graceful shutdown.
func (s *Service[T]) Run() error {
	if len(s.transports) == 0 {
		s.cleanup()
		return fmt.Errorf("service: at least one transport is required")
	}

	startupCtx := context.Background()
	for i, fn := range s.onStartup {
		if err := fn(startupCtx); err != nil {
			s.cleanup()
			return fmt.Errorf("startup hook %d: %w", i, err)
		}
	}

	ready := make(chan struct{})
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	errCh := s.startTransports(ready)

	select {
	case err := <-errCh:
		s.cleanup()
		return fmt.Errorf("transport serve: %w", err)
	case <-ready:
	case <-exit:
		s.log.Info("shutdown signal received before ready")
		return s.shutdown()
	}

	select {
	case err := <-errCh:
		s.log.Error(err, "transport stopped unexpectedly")
		return s.shutdown()
	case <-exit:
		return s.shutdown()
	}
}

func (s *Service[T]) startTransports(ready chan<- struct{}) <-chan error {
	errCh := make(chan error, len(s.transports))
	var readyCount atomic.Int32
	expected := int32(len(s.transports))

	for _, tr := range s.transports {
		go func() {
			errCh <- tr.Serve(transport.ServeOptions{
				OnReady: func() {
					if readyCount.Add(1) == expected {
						close(ready)
					}
				},
			})
		}()
	}

	return errCh
}

func (s *Service[T]) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()

	s.log.Info("graceful shutdown started")

	for i := len(s.onShutdown) - 1; i >= 0; i-- {
		if err := s.onShutdown[i](ctx); err != nil {
			s.log.Error(err, "shutdown hook failed", fmt.Sprintf("hook=%d", i))
		}
	}

	for i := len(s.transports) - 1; i >= 0; i-- {
		tr := s.transports[i]
		if err := tr.Shutdown(ctx); err != nil {
			s.log.Error(err, "transport shutdown failed", tr.Name())
		}
	}

	s.cleanup()
	s.log.Info("graceful shutdown completed")
	return nil
}

func (s *Service[T]) cleanup() {
	if s.ownsLogger {
		s.log.Close()
	}
}
