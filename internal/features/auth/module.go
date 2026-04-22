package auth

import (
	"database/sql"
	"net/http"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/infra/cache"
	"github.com/LuizFernando991/gym-api/internal/infra/email"
	sharedmiddleware "github.com/LuizFernando991/gym-api/internal/shared/middleware"
	"github.com/gorilla/mux"
)

type Module struct {
	handler    *Handler
	middleware func(http.Handler) http.Handler
}

func NewModule(db *sql.DB, cfg config.AuthConfig, emailSender email.Sender, rateLimiter cache.RateLimiter) *Module {
	repo := NewRepository(db)
	svc := NewService(repo, cfg, emailSender, rateLimiter)
	return &Module{
		handler:    NewHandler(svc),
		middleware: sharedmiddleware.Auth(svc),
	}
}

func (m *Module) RegisterRoutes(r *mux.Router) {
	m.handler.RegisterRoutes(r, m.middleware)
}
