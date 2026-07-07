// Run: go run ./examples/multiport
//
// One runtime, two HTTP units on different ports. Multi-port is composition
// of units — not a special multi-bind feature on a single transport.
//
//	curl http://127.0.0.1:8080/
//	curl http://127.0.0.1:8081/
package main

import (
	"log"

	"github.com/InTacht/xqua-go/examples/multiport/transport"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/runtime"
)

type Ctx struct {
	Name string
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	appLog := logger.New(&logger.Config{Name: "multiport", ID: "multiport"})
	defer appLog.Close()

	r, err := runtime.New(&Ctx{Name: "multiport"}, appLog)
	if err != nil {
		return err
	}

	r.Unit(func(c *Ctx, log runtime.Logger) runtime.Unit {
		return transport.Public(c.Name, log.Derive("public"))
	}).Unit(func(c *Ctx, log runtime.Logger) runtime.Unit {
		return transport.Admin(c.Name, log.Derive("admin"))
	})

	return r.Run()
}
