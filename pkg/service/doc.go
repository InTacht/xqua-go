// Package service orchestrates process lifecycle for xqua-go applications.
//
// # Configuration
//
// Config requires Name and ID. ShutdownTimeout defaults to 30 seconds when
// zero or unset. Debug enables debug logging on a service-created logger.
// Version, BuildID, and BuildTime are optional metadata. When Logger is nil,
// New creates one from Name, ID, and Debug.
//
// # Application context
//
// Service is generic over ctx.Ctx. New calls ctx.Build during startup; a Build
// failure panics. Pass your application context alongside Config:
//
//	srv := service.New(service.Config{
//	    Name: "orders",
//	    ID:   "orders-api",
//	}, &appctx.Ctx{})
//
// New panics when Name or ID is missing or when ctx.Build fails.
//
// # Transports
//
// Register transports with Transport, passing a CreateTransportFunc that
// receives the application context and service logger. The factory runs
// immediately; nil factories are skipped. Run starts every registered transport
// concurrently and waits for all of them to report ready.
//
//	srv.Transport(transport.HTTP)
//
// # Lifecycle hooks
//
// OnStartup hooks run in registration order before transports serve. A failing
// hook aborts Run and triggers cleanup.
//
// OnShutdown hooks run in reverse registration order after SIGINT or SIGTERM,
// before transport shutdown. Use them to drain in-flight work while transports
// are still running. Hook errors are logged; shutdown continues. Transports
// shut down in reverse registration order. Shutdown uses ShutdownTimeout from
// config.
//
// # Run
//
// Run requires at least one transport. Startup hook errors and transport serve
// errors before ready abort Run and call cleanup. An unexpected transport stop
// after ready triggers graceful shutdown. When the service created the logger,
// cleanup closes it on exit.
//
// # Quick reference
//
//	Operation       Use when
//	---------       --------
//	New             Create a service from config and app context
//	Transport       Register a transport factory
//	OnStartup       Run logic before serving
//	OnShutdown      Drain or prepare shutdown before transports stop
//	Logger          Access the service logger
//	Run             Start hooks, serve transports, shut down on signal
package service
