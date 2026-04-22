package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/LuizFernando991/gym-api/internal/shared/entities"
	"github.com/LuizFernando991/gym-api/internal/shared/httputil"
)

type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*entities.Session, error)
}

func Auth(v TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				httputil.Error(w, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")
			session, err := v.ValidateToken(r.Context(), token)
			if err != nil {
				httputil.Error(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			next.ServeHTTP(w, r.WithContext(httputil.ContextWithSession(r.Context(), session)))
		})
	}
}
