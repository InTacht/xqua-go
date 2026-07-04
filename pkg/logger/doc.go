// Package logger wraps zap with XQUA logging conventions.
//
// # Configuration
//
// Create a Logger with New or wrap an existing zap logger with FromZap.
// Config supplies service name, id, label, debug level, and request ID field
// names for HTTP propagation:
//
//	appLog := logger.New(&logger.Config{
//	    Name:  "orders",
//	    ID:    "orders-api",
//	    Label: "server",
//	    Debug: true,
//	})
//	defer appLog.Close() // root only
//
// Derive returns a child logger (as runtime.Logger) whose label is extended
// (parent.child) while sharing the same underlying zap core. Children are
// cheap; never Close them — only the root from New/FromZap is Closed. Use
// Derive from unit factories to scope logs per component without holding the
// concrete *Logger.
//
// # Logging methods
//
// Debug, Info, Warn, and Error accept message segments joined with ". ".
// Context variants (DebugCtx, InfoCtx, …) attach request_id from ctx when
// present. Wrap variants log and return (nil for non-error levels; the error
// argument unchanged for ErrorWrap).
//
// # Request ID
//
// Attach a request ID to context with ContextWithRequestID. Context-aware
// methods read it via RequestID and emit it under RequestIDFieldKey
// (default "request_id").
//
// # Error fields
//
// Error and ErrorCtx encode pkg/errors values as structured fields:
//
//   - Structured errors: "errors" array of objects (kind, code, message,
//     source, and cause when wrapped).
//   - Plain error: "error" string via zap.Error.
//
// Log one layer per call; rebuild deeper traces from ordered log lines that
// share the same request_id.
//
// # Quick reference
//
//	Operation              Use when
//	---------              --------
//	New / FromZap          Create or wrap a root logger
//	Derive                 Scope logs to a sub-component label (shared core)
//	ContextWithRequestID   Propagate request ID into ctx
//	Debug/Info/Warn/Error  Log at a level
//	*Ctx                   Same, with request_id from ctx
//	*Wrap / ErrorWrap      Log and return (error levels return the error)
//	Zap                    Access underlying zap for middleware
//	Close                  Flush the root only (not derived children)
package logger
