package leveling

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// XPAwarder is the interface workout (and future features) use to grant XP
// without importing the leveling package directly.
type XPAwarder interface {
	AwardWorkoutCompletion(ctx context.Context, userID, sessionID string, sessionDate time.Time) error
}

type Module struct {
	handler *Handler
	svc     Service
}

func NewModule(db *sql.DB) *Module {
	repo := NewRepository(db)
	svc := NewService(repo)
	return &Module{handler: NewHandler(svc), svc: svc}
}

func (m *Module) RegisterRoutes(r *mux.Router, authMiddleware func(http.Handler) http.Handler) {
	m.handler.RegisterRoutes(r, authMiddleware)
}

// Awarder returns the XPAwarder interface backed by this module's service.
func (m *Module) Awarder() XPAwarder {
	return m.svc
}