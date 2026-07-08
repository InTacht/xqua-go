# Multi-unit apps

Run more than one long-lived component in a single process — or split them later with the same bus API.

## The Unit interface

Anything that listens or works in the background implements:

```go
type Unit interface {
    Name() string
    Serve(opts ServeOptions) error
    Shutdown(ctx context.Context) error
}
```

`pkg/http.Transport` is a `Unit`. So is a custom worker you write.

## Register units

```go
type App struct {
    Bus bus.Bus
    // pools, services, …
}

r, _ := runtime.New(app, log)

r.Unit(func(app *App, log runtime.Logger) runtime.Unit {
    return http.New(http.Config{Logger: log.Derive("http"), Catalog: api, ...})
})

r.Unit(func(app *App, log runtime.Logger) runtime.Unit {
    return worker.New(app.Bus, log.Derive("worker"))
})

r.Run()
```

Factories receive your app context and logger — pull out only what each unit needs.

## Lifecycle

| Phase | What happens |
| ----- | ------------ |
| `OnStartup` hooks | Run in order (migrations, warm caches) — failure aborts start |
| `Serve` | Each unit runs in its own goroutine |
| Signal / unit crash | Graceful shutdown starts |
| `OnShutdown` hooks | Reverse order — drain work while units still up |
| `Unit.Shutdown` | Reverse registration order, default 30s timeout |
| `main` defers | You close pools, bus, etc. |

```go
r.OnShutdown(func(ctx context.Context) error {
    return app.Bus.Drain(ctx)
})
```

## Message bus between units

Create the bus in `main`, store on app context:

```go
b := bus.NewLocal(bus.LocalConfig{})
app := &App{Bus: b}
```

### Publish / subscribe

```go
sub, _ := app.Bus.Subscribe("orders.created", func(ctx context.Context, msg bus.Message) error {
    // msg.Data is []byte — decode JSON yourself
    return nil
})
defer sub.Unsubscribe()

app.Bus.Publish(ctx, bus.Message{Subject: "orders.created", Data: payload})
```

### Competing workers

```go
app.Bus.QueueSubscribe("jobs.run", "workers", handler)
// add more QueueSubscribe with same queue name = more consumers
```

### Request / reply

```go
resp, err := app.Bus.Request(ctx, bus.Message{
    Subject: "storage.get",
    Data:    reqJSON,
}, timeout)
```

Handlers must set `msg.Reply` on the incoming message when using request/reply (see `examples/bus`).

## Examples

| Example | Pattern |
| ------- | ------- |
| [`multiport`](../examples/multiport) | Two HTTP servers, two ports, one runtime |
| [`bus`](../examples/bus) | HTTP unit sends bus requests to queue workers |
| [`split`](../examples/split) | Compute unit + storage unit, no shared Go pointers |

## Rules of thumb

- Units in one process share memory **only** through your app context — not as a public API between packages.
- For cross-language or separate deployables later, use the same bus subjects and JSON payloads you'd want on the wire anyway.
- Always `Drain` the bus on shutdown if handlers must finish in-flight work.
