package group

import (
	"database/sql"
	"net/http"

	"github.com/LuizFernando991/gym-api/internal/infra/storage"
	"github.com/LuizFernando991/gym-api/internal/shared/httputil"
	"github.com/gorilla/mux"
)

type Module struct {
	handler *Handler
}

func NewModule(db *sql.DB, uploader storage.Uploader, rl httputil.RateAllower, notifier Notifier) *Module {
	repo := NewRepository(db)
	svc := NewService(repo, uploader, notifier)
	return &Module{handler: NewHandler(svc, rl)}
}

func (m *Module) RegisterRoutes(r *mux.Router, authMiddleware func(http.Handler) http.Handler) {
	m.handler.RegisterRoutes(r, authMiddleware)
}
