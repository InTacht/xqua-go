// Run: go run ./examples/hello
//
// Minimal xqua-go service: app context, HTTP transport factory, routes, and Run.
package main

import (
	"log"
	"os"

	"github.com/InTacht/xqua-go/examples/hello/ctx"
	"github.com/InTacht/xqua-go/examples/hello/transport"

	"github.com/InTacht/xqua-go/pkg/service"
)

func main() {
	srv := service.New(service.Config{
		Name: "hello",
		ID:   "hello",
	}, &ctx.Ctx{}).Transport(transport.HTTP)

	if err := srv.Run(); err != nil {
		log.Printf("service stopped: %v", err)
		os.Exit(1)
	}
}
