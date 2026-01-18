package identity

import (
	"context"

	"github.com/gruzdev-dev/codex-files/core/domain"
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
