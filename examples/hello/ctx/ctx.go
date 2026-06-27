package ctx

import (
	"context"
	"fmt"
)

type Ctx struct {
	DB1 string
}

func (c *Ctx) Build(ctx context.Context) error {
	fmt.Println("Building context")
	return nil
}

func (c *Ctx) Destroy(ctx context.Context) error {
	fmt.Println("Destroying context")
	return nil
}
