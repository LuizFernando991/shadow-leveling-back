package httputil

import (
	"context"

	"github.com/LuizFernando991/gym-api/internal/shared/entities"
)

type contextKey string

const sessionContextKey contextKey = "session"

func ContextWithSession(ctx context.Context, s *entities.Session) context.Context {
	return context.WithValue(ctx, sessionContextKey, s)
}

func SessionFromContext(ctx context.Context) *entities.Session {
	s, _ := ctx.Value(sessionContextKey).(*entities.Session)
	return s
}
