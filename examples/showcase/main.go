package main

import (
	"log"

	"github.com/InTacht/xqua-go/examples/showcase/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("showcase: %v", err)
	}
}
