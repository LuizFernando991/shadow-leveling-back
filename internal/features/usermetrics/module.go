package usermetrics

import (
	"database/sql"
	"net/http"

	"github.com/LuizFernando991/gym-api/internal/features/task"
	"github.com/LuizFernando991/gym-api/internal/features/workout"
	"github.com/gorilla/mux"
)

type Module struct {
	handler *Handler
}

func NewModule(db *sql.DB) *Module {
	workoutRepo := workout.NewRepository(db)
	workoutSvc := workout.NewService(workoutRepo)

	taskRepo := task.NewRepository(db)
	taskSvc := task.NewService(taskRepo)

	svc := NewService(workoutSvc, taskSvc)
	return &Module{handler: NewHandler(svc)}
}

func (m *Module) RegisterRoutes(r *mux.Router, authMiddleware func(http.Handler) http.Handler) {
	m.handler.RegisterRoutes(r, authMiddleware)
}
