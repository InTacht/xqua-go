package ctx

import "context"

// Ctx manages shared context for a service.
type Ctx interface {
	Build(ctx context.Context) error
	Destroy(ctx context.Context) error
}
