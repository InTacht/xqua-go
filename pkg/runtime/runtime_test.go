package runtime_test

import (
	"context"
	"testing"

	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/runtime"
)

// mockCtx is a plain application context. Runtime never builds or tears it down.
type mockCtx struct {
	name string
}

type stubUnit struct {
	ready bool
}

func (stubUnit) Name() string { return "stub" }

func (u *stubUnit) Serve(opts runtime.ServeOptions) error {
	if opts.OnReady != nil {
		opts.OnReady()
	}
	u.ready = true
	<-context.Background().Done()
	return nil
}

func (stubUnit) Shutdown(context.Context) error { return nil }

func testLogger() runtime.Logger {
	return logger.New(&logger.Config{Name: "test", ID: "test-1"})
}

func newRuntime(t *testing.T, app *mockCtx) *runtime.Runtime[*mockCtx] {
	t.Helper()
	r, err := runtime.New(app, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

func TestRunWithoutUnitReturnsError(t *testing.T) {
	r := newRuntime(t, &mockCtx{})

	if err := r.Run(); err == nil {
		t.Fatal("expected error when no unit registered")
	}
}

func TestRunWithShutdownTimeoutWithoutUnitReturnsError(t *testing.T) {
	r := newRuntime(t, &mockCtx{})

	// Zero timeout falls back to DefaultShutdownTimeout; still requires a unit.
	if err := r.RunWithShutdownTimeout(0); err == nil {
		t.Fatal("expected error when no unit registered")
	}
}

func TestStartupHookFailureAbortsRun(t *testing.T) {
	r := newRuntime(t, &mockCtx{})
	r.OnStartup(func(context.Context) error {
		return context.Canceled
	})
	r.Unit(func(app *mockCtx, _ runtime.Logger) runtime.Unit {
		return &stubUnit{}
	})

	if err := r.Run(); err == nil {
		t.Fatal("expected startup hook error")
	}
}

func TestNewRejectsMissingLogger(t *testing.T) {
	if _, err := runtime.New(&mockCtx{}, nil); err == nil {
		t.Fatal("expected error when Logger is nil")
	}
}

func TestNewPassesContextToUnitFactory(t *testing.T) {
	app := &mockCtx{name: "app-1"}
	r := newRuntime(t, app)

	var got *mockCtx
	r.Unit(func(c *mockCtx, _ runtime.Logger) runtime.Unit {
		got = c
		return &stubUnit{}
	})

	if got != app {
		t.Fatalf("unit factory received %v, want %v", got, app)
	}
}

func TestUnitFactoryReceivesDerivedLogger(t *testing.T) {
	app := &mockCtx{name: "app-1"}
	r := newRuntime(t, app)

	var got runtime.Logger
	r.Unit(func(_ *mockCtx, log runtime.Logger) runtime.Unit {
		got = log.Derive("stub")
		return &stubUnit{}
	})

	if got == nil {
		t.Fatal("expected derived logger")
	}
	if _, ok := got.(*logger.Logger); !ok {
		t.Fatalf("derived logger type %T, want *logger.Logger", got)
	}
}
