package runtime

import (
	"context"
)

// Unit is a long-lived supervisee such as an HTTP server, job worker, or agent.
// Implementations live in their own packages (for example pkg/http) and are
// registered with Runtime via CreateUnitFunc.
type Unit interface {
	Name() string
	Serve(opts ServeOptions) error
	Shutdown(ctx context.Context) error
}

// ServeOptions configures unit startup behavior.
type ServeOptions struct {
	OnReady func()
}

// CreateUnitFunc builds a Unit from the application context and runtime logger.
// The factory runs when Unit is called on Runtime, not when Run starts. It is
// the narrowing point: pull the specific dependencies a unit needs out of the
// application context here, so unit packages never depend on your ctx type.
type CreateUnitFunc[T any] func(ctx T, logger Logger) Unit
