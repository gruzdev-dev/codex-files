package identity

import (
	"codex-files/core/domain"
	"context"
)

type ctxKey int

const identityKey ctxKey = iota

func WithCtx(ctx context.Context, id domain.Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

func FromCtx(ctx context.Context) (domain.Identity, bool) {
	id, ok := ctx.Value(identityKey).(domain.Identity)
	return id, ok
}
