package auth

import (
	"context"
	"crypto/sha256"
)

type Identity struct {
	Reference [sha256.Size]byte
}

type identityContextKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok
}

func identityFromOpenID(openID string) Identity {
	return Identity{Reference: sha256.Sum256([]byte(openID))}
}
