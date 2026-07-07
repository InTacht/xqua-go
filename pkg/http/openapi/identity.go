package openapi

import (
	"context"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/wire"
)

// Identity is the authenticated subject attached to a secured request context.
// The engine stores whatever Verify returns; handlers choose how to interpret it.
type Identity = any

// WithIdentity attaches id to ctx after successful authentication.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return wire.WithIdentity(ctx, id)
}

// IdentityFrom returns the identity stored in ctx by the security guard.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	return wire.IdentityFrom(ctx)
}

// IdentityAs returns the identity stored in ctx as T when the dynamic type matches.
func IdentityAs[T any](ctx context.Context) (T, bool) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		var zero T
		return zero, false
	}
	v, ok := id.(T)
	return v, ok
}
