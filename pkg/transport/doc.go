// Package transport defines the network transport lifecycle contract.
//
// Transport implementations (HTTP, gRPC, etc.) expose Name, Serve, and Shutdown.
// Serve blocks until Shutdown is called or an unrecoverable error occurs.
// OnReady in ServeOptions signals that the transport is accepting work.
//
// Register transports with service.Service via CreateTransportFunc. The factory
// receives the application context and service logger so each transport can
// assemble routes and dependencies without the service knowing protocol details:
//
//	func HTTP(app *myctx.Ctx, log *logger.Logger) transport.Transport {
//	    return http.New(http.Config{
//	        Host: app.Host, Port: app.Port, Logger: log,
//	    }).Routes("/api", registerRoutes)
//	}
//
//	srv.Transport(HTTP)
package transport
