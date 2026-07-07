package wire

import "context"

type identityKeyType struct{}

var identityContextKey = identityKeyType{}

// WithIdentity attaches id to ctx after successful authentication.
func WithIdentity(ctx context.Context, id any) context.Context {
	return context.WithValue(ctx, identityContextKey, id)
}

// IdentityFrom returns the identity stored in ctx.
func IdentityFrom(ctx context.Context) (any, bool) {
	if ctx == nil {
		return nil, false
	}
	id, ok := ctx.Value(identityContextKey).(any)
	if !ok || id == nil {
		return nil, false
	}
	return id, true
}
