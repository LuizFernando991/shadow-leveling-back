package auth

import (
	"database/sql"
	"net/http"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/infra/email"
	"github.com/LuizFernando991/gym-api/internal/infra/storage"
	"github.com/LuizFernando991/gym-api/internal/shared/httputil"
	sharedmiddleware "github.com/LuizFernando991/gym-api/internal/shared/middleware"
	"github.com/gorilla/mux"
)

type Module struct {
	handler    *Handler
	svc        Service
	middleware func(http.Handler) http.Handler
}

func NewModule(db *sql.DB, cfg config.AuthConfig, sender email.Sender, limiter httputil.RateAllower, verifier TokenVerifier, uploader storage.Uploader) *Module {
	repo := NewRepository(db)
	svc := NewService(repo, cfg, sender, verifier, uploader)
	return &Module{
		handler:    NewHandler(svc, limiter),
		svc:        svc,
		middleware: sharedmiddleware.Auth(svc),
	}
}

func (m *Module) RegisterRoutes(r *mux.Router) {
	m.handler.RegisterRoutes(r, m.middleware)
}

func (m *Module) Middleware() func(http.Handler) http.Handler {
	return m.middleware
}

// GoalReader returns the GoalReader backed by this module's service, for
// other features (e.g. usermetrics) to read per-user weekly goal settings.
func (m *Module) GoalReader() GoalReader {
	return m.svc
}
