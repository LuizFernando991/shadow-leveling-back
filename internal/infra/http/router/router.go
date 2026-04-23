package router

import (
	"net/http"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/features/auth"
	"github.com/LuizFernando991/gym-api/internal/features/task"
	"github.com/LuizFernando991/gym-api/internal/features/workout"
	"github.com/LuizFernando991/gym-api/internal/infra/http/docs"
	"github.com/LuizFernando991/gym-api/internal/infra/http/middleware"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Modules struct {
	Auth    *auth.Module
	Task    *task.Module
	Workout *workout.Module
}

func NewRouter(cfg *config.Config, modules Modules) http.Handler {
	r := mux.NewRouter()

	r.Use(middleware.CORS)
	r.Use(middleware.JSONContentType)
	r.Use(middleware.Logger)

	if cfg.App.MetricsEnabled {
		r.Use(middleware.MetricsMiddleware)
	}

	r.NotFoundHandler = http.HandlerFunc(notFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(methodNotAllowed)

	registerRoutes(r, cfg, modules)

	return r
}

func registerRoutes(r *mux.Router, cfg *config.Config, modules Modules) {
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods(http.MethodGet)

	if cfg.App.MetricsEnabled {
		r.Handle("/metrics", promhttp.Handler()).Methods(http.MethodGet)
	}

	modules.Auth.RegisterRoutes(r)
	modules.Task.RegisterRoutes(r, modules.Auth.Middleware())
	modules.Workout.RegisterRoutes(r, modules.Auth.Middleware())
	docs.RegisterRoutes(r)
}

func notFound(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
}
