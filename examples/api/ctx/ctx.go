package ctx

import (
	"context"
	"fmt"
)

// Ctx holds application dependencies and runtime configuration.
type Ctx struct {
	Name string
	Host string
	Port int
}

func (c *Ctx) Build(ctx context.Context) error {
	fmt.Println("building application context")
	return nil
}

func (c *Ctx) Destroy(ctx context.Context) error {
	fmt.Println("destroying application context")
	return nil
}
