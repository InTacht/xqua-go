// Run: go run ./examples/catalog
//
// Public error catalog discipline over HTTP: kind→status defaults, internal
// store catalog mapped at the boundary, and leak protection for foreign errors.
//
//	curl http://127.0.0.1:8080/items/1
//	curl http://127.0.0.1:8080/items/9
//	curl http://127.0.0.1:8080/items/99
//	curl -X POST http://127.0.0.1:8080/items
//	curl http://127.0.0.1:8080/leak
//	curl http://127.0.0.1:8080/plain
package main

import (
	"log"
	"os"

	"github.com/InTacht/xqua-go/examples/catalog/transport"
	"github.com/InTacht/xqua-go/pkg/logger"
	"github.com/InTacht/xqua-go/pkg/runtime"
)

// Ctx is a minimal app context (no external deps). Runtime carries it to unit
// factories but never builds or tears it down.
type Ctx struct{}

func main() {
	appLog := logger.New(&logger.Config{Name: "catalog", ID: "catalog"})
	defer appLog.Close()

	r, err := runtime.New(&Ctx{}, appLog)
	if err != nil {
		log.Printf("runtime init: %v", err)
		os.Exit(1)
	}
	r.Unit(func(_ *Ctx, log runtime.Logger) runtime.Unit {
		return transport.HTTP(log.Derive("http"))
	})

	if err := r.Run(); err != nil {
		log.Printf("runtime stopped: %v", err)
		os.Exit(1)
	}
}
