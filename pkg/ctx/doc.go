// Package ctx defines the application context contract for xqua-go services.
//
// Implement Ctx on a type your service owns (typically a struct with databases,
// clients, and configuration). Service.New calls Build during startup; use it
// to open connections and wire dependencies. Destroy is available for explicit
// teardown when your application calls it.
//
//	type Ctx struct {
//	    DB *sql.DB
//	}
//
//	func (c *Ctx) Build(ctx context.Context) error {
//	    db, err := sql.Open(...)
//	    c.DB = db
//	    return err
//	}
//
//	func (c *Ctx) Destroy(ctx context.Context) error {
//	    return c.DB.Close()
//	}
//
// Pass the context instance to service.New alongside Config. Transports receive
// the same instance when registered via CreateTransportFunc.
package ctx
