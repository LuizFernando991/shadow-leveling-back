package auth

import (
	"database/sql"
	"net/http"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/infra/email"
	"github.com/LuizFernando991/gym-api/internal/shared/httputil"
	sharedmiddleware "github.com/LuizFernando991/gym-api/internal/shared/middleware"
	"github.com/gorilla/mux"
)

type Module struct {
	handler    *Handler
	middleware func(http.Handler) http.Handler
}

func NewModule(db *sql.DB, cfg config.AuthConfig, sender email.Sender, limiter httputil.RateAllower, verifier TokenVerifier) *Module {
	repo := NewRepository(db)
	svc := NewService(repo, cfg, sender, verifier)
	return &Module{
		handler:    NewHandler(svc, limiter),
		middleware: sharedmiddleware.Auth(svc),
	}
}

func (m *Module) RegisterRoutes(r *mux.Router) {
	m.handler.RegisterRoutes(r, m.middleware)
}

func (m *Module) Middleware() func(http.Handler) http.Handler {
	return m.middleware
}
