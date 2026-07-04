package runtime

import "context"

// Logger is the headless logging surface used by Runtime and units.
// Implementations such as *logger.Logger satisfy it. The runtime never
// constructs or closes a logger — main owns the root (logger.New +
// defer appLog.Close()).
//
// Derive returns a child logger scoped with an extended label (or self when
// the implementation is a no-op). Children share the root backend and must
// not be Closed. Unit factories typically call Derive at the registration
// site so each unit logs under its own label.
type Logger interface {
	Info(msg ...string)
	InfoCtx(ctx context.Context, msg ...string)
	Warn(msg ...string)
	WarnCtx(ctx context.Context, msg ...string)
	Error(err error, msg ...string)
	ErrorCtx(ctx context.Context, err error, msg ...string)
	Derive(label string) Logger
}
