package api

import (
	"context"
	"net/http"

	"capper/internal/authz"
)

type ctxKey int

const (
	principalTypeKey ctxKey = iota
	principalIDKey
	authContextKey
)

func withPrincipal(ctx context.Context, principalType, principalID string) context.Context {
	ctx = context.WithValue(ctx, principalTypeKey, principalType)
	ctx = context.WithValue(ctx, principalIDKey, principalID)
	return ctx
}

func principalFromContext(ctx context.Context) (string, string) {
	pt, _ := ctx.Value(principalTypeKey).(string)
	pid, _ := ctx.Value(principalIDKey).(string)
	return pt, pid
}

func withAuthContext(ctx context.Context, ac authz.AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey, ac)
}

func authContextFrom(r *http.Request) authz.AuthContext {
	ac, _ := r.Context().Value(authContextKey).(authz.AuthContext)
	return ac
}
