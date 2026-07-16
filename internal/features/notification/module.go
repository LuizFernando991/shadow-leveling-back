package notification

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/LuizFernando991/gym-api/internal/infra/push"
	"github.com/gorilla/mux"
)

// Notifier is the interface other features use to trigger push notifications
// without importing this package directly (mirrors leveling.Awarder). Its method
// set is a superset of workout.GroupNotifier and group.Notifier.
type Notifier interface {
	NotifyWorkoutCompleted(ctx context.Context, userID string, sessionDate time.Time)
	NotifySessionReaction(ctx context.Context, actorID, sessionID string)
	NotifySessionComment(ctx context.Context, actorID, sessionID string)
}

type Module struct {
	handler *Handler
	svc     Service
}

func NewModule(db *sql.DB, sender push.Sender) *Module {
	repo := NewRepository(db)
	svc := NewService(repo, sender)
	return &Module{handler: NewHandler(svc), svc: svc}
}

func (m *Module) RegisterRoutes(r *mux.Router, authMiddleware func(http.Handler) http.Handler) {
	m.handler.RegisterRoutes(r, authMiddleware)
}

// Notifier returns the workout-facing notifier backed by this module's service.
func (m *Module) Notifier() Notifier {
	return m.svc
}
