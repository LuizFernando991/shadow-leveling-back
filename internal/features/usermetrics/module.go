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

func NewModule(db *sql.DB, goalReader GoalReader) *Module {
	workoutRepo := workout.NewRepository(db)
	workoutSvc := workout.NewService(workoutRepo, nil, nil, nil)

	taskRepo := task.NewRepository(db)
	taskSvc := task.NewService(taskRepo)

	svc := NewService(workoutSvc, taskSvc, goalReader)
	return &Module{handler: NewHandler(svc)}
}

func (m *Module) RegisterRoutes(r *mux.Router, authMiddleware func(http.Handler) http.Handler) {
	m.handler.RegisterRoutes(r, authMiddleware)
}
