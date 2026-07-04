package runtime

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

// DefaultShutdownTimeout is used by Run when no explicit timeout is given.
const DefaultShutdownTimeout = 30 * time.Second

// LifecycleHook runs during runtime startup or shutdown.
type LifecycleHook func(ctx context.Context) error

// Runtime orchestrates lifecycle hooks and registered units. T is the caller's
// application context — any value holding shared dependencies. Runtime passes it
// to unit factories but never builds or tears it down; the caller owns that.
type Runtime[T any] struct {
	ctx T
	log Logger

	units []Unit

	onStartup  []LifecycleHook
	onShutdown []LifecycleHook
}

// New creates a Runtime with the given application context and logger
// (ctx first). It returns an error when log is nil. The application context
// and root logger are owned by the caller (in main); Runtime never closes log.
func New[T any](ctx T, log Logger) (*Runtime[T], error) {
	if log == nil {
		return nil, fmt.Errorf("runtime: Logger is required")
	}

	return &Runtime[T]{
		ctx: ctx,
		log: log,
	}, nil
}

// Logger returns the runtime logger.
func (r *Runtime[T]) Logger() Logger {
	return r.log
}

// Unit registers a unit to be started when Run is called.
func (r *Runtime[T]) Unit(fn CreateUnitFunc[T]) *Runtime[T] {
	if fn != nil {
		r.units = append(r.units, fn(r.ctx, r.log))
	}
	return r
}

// OnStartup registers a hook that runs before units start serving.
func (r *Runtime[T]) OnStartup(fn LifecycleHook) *Runtime[T] {
	r.onStartup = append(r.onStartup, fn)
	return r
}

// OnShutdown registers a hook that runs after a stop signal and before unit shutdown.
// Hooks run in reverse registration order.
func (r *Runtime[T]) OnShutdown(fn LifecycleHook) *Runtime[T] {
	r.onShutdown = append(r.onShutdown, fn)
	return r
}

// Run executes startup hooks, serves all registered units, and performs a
// graceful shutdown with DefaultShutdownTimeout.
func (r *Runtime[T]) Run() error {
	return r.RunWithShutdownTimeout(DefaultShutdownTimeout)
}

// RunWithShutdownTimeout is like Run but bounds graceful shutdown to timeout.
// When timeout is zero or negative, DefaultShutdownTimeout is used.
func (r *Runtime[T]) RunWithShutdownTimeout(timeout time.Duration) error {
	if timeout <= 0 {
		timeout = DefaultShutdownTimeout
	}

	if len(r.units) == 0 {
		return fmt.Errorf("runtime: at least one unit is required")
	}

	startupCtx := context.Background()
	for i, fn := range r.onStartup {
		if err := fn(startupCtx); err != nil {
			return fmt.Errorf("startup hook %d: %w", i, err)
		}
	}

	ready := make(chan struct{})
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	errCh := r.startUnits(ready)

	select {
	case err := <-errCh:
		return fmt.Errorf("unit serve: %w", err)
	case <-ready:
	case <-exit:
		r.log.Info("shutdown signal received before ready")
		return r.shutdown(timeout)
	}

	select {
	case err := <-errCh:
		r.log.Error(err, "unit stopped unexpectedly")
		return r.shutdown(timeout)
	case <-exit:
		return r.shutdown(timeout)
	}
}

func (r *Runtime[T]) startUnits(ready chan<- struct{}) <-chan error {
	errCh := make(chan error, len(r.units))
	var readyCount atomic.Int32
	expected := int32(len(r.units))

	for _, u := range r.units {
		go func() {
			errCh <- u.Serve(ServeOptions{
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

func (r *Runtime[T]) shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	r.log.Info("graceful shutdown started")

	for i := len(r.onShutdown) - 1; i >= 0; i-- {
		if err := r.onShutdown[i](ctx); err != nil {
			r.log.Error(err, "shutdown hook failed", fmt.Sprintf("hook=%d", i))
		}
	}

	for i := len(r.units) - 1; i >= 0; i-- {
		u := r.units[i]
		if err := u.Shutdown(ctx); err != nil {
			r.log.Error(err, "unit shutdown failed", u.Name())
		}
	}

	r.log.Info("graceful shutdown completed")
	return nil
}
