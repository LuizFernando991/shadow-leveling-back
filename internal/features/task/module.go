package task

import (
	"database/sql"
	"net/http"

	"github.com/gorilla/mux"
)

type Module struct {
	handler *Handler
}

func NewModule(db *sql.DB) *Module {
	repo := NewRepository(db)
	svc := NewService(repo)
	return &Module{handler: NewHandler(svc)}
}

func (m *Module) RegisterRoutes(r *mux.Router, authMiddleware func(http.Handler) http.Handler) {
	m.handler.RegisterRoutes(r, authMiddleware)
}
