// Package runtime is a headless process supervisor: supervised units, lifecycle
// hooks, and graceful shutdown. It stands alone — wire your units (HTTP, jobs,
// agents, …), hooks, and Logger yourself. This package does not assemble an
// application for you, does not import protocol stacks, and does not construct
// or close a logger.
//
// Import path: github.com/InTacht/xqua-go/pkg/runtime
//
// # Application context
//
// Runtime carries an application context of any type T — a value your program
// owns that holds shared dependencies (databases, clients, buses, config).
// Runtime does not build or tear it down: you construct dependencies in main
// and release them there too (defer after each successful open). This keeps
// runtime pure mechanism — there is no Build/Destroy contract to satisfy.
//
//	// main owns the full lifecycle
//	pool, err := pgxpool.New(ctx, dsn)
//	if err != nil { /* … */ }
//	defer pool.Close()
//
//	app := &App{Pool: pool, Users: store.NewUsers(pool)}
//	r, err := runtime.New(app, log)
//	if err != nil { /* … */ }
//
// # Units
//
// A Unit is any long-lived supervisee: Name, Serve, Shutdown. Protocol packages
// such as pkg/http implement Unit. Register factories with Unit; each receives
// the application context and runtime logger. The factory is the narrowing
// point — pull out only the dependencies a unit needs so unit packages never
// import your context type.
//
//	r.Unit(func(app *App, log runtime.Logger) runtime.Unit {
//	    return worker.New(app.Bus, log.Derive("worker")) // narrowed + labeled
//	})
//	if err := r.Run(); err != nil { /* … */ }
//
// # Lifecycle hooks
//
// OnStartup hooks run in registration order before units serve. A failing hook
// aborts Run.
//
// OnShutdown hooks run in reverse registration order after SIGINT or SIGTERM,
// before unit shutdown. Use them to drain in-flight work while units are still
// running. Hook errors are logged; shutdown continues. Units shut down in
// reverse registration order. Application resources are released by the caller
// in main, not by runtime.
//
// # Logger
//
// New(ctx, log) requires a non-nil Logger (ctx first). There is no default
// logger: pass an implementation of Logger (for example *logger.Logger). Main
// owns the root lifecycle (logger.New + defer Close); Runtime never closes it.
// Derive returns a child for per-unit labels and shares the root backend —
// call it in unit factories at registration, and never Close children.
// Process identity (name, instance id) lives on the logger, not here. Shutdown
// timeout is not configuration — pass it to RunWithShutdownTimeout when the
// default is wrong.
//
// # Run
//
// New returns an error when log is nil. Run requires at least one unit and uses
// DefaultShutdownTimeout (30s). RunWithShutdownTimeout is the same with an
// explicit bound; zero or negative falls back to the default. Startup hook
// errors and unit serve errors before ready abort Run. An unexpected unit stop
// after ready triggers graceful shutdown.
//
// # Quick reference
//
//	Operation                 Use when
//	---------                 --------
//	New                       Create a runtime from app context and logger
//	Unit                      Register a unit factory
//	OnStartup                 Run logic before serving
//	OnShutdown                Drain or prepare shutdown before units stop
//	Logger                    Access the runtime logger
//	Logger.Derive             Scope a child logger label for a unit
//	Run                       Start hooks, serve units, shut down (30s)
//	RunWithShutdownTimeout    Same as Run with an explicit shutdown bound
package runtime
