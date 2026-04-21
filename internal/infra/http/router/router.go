package router

import (
	"net/http"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/infra/http/middleware"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handlers struct {
}

func NewRouter(cfg *config.Config, handlers Handlers) http.Handler {
	r := mux.NewRouter()

	r.Use(middleware.CORS)
	r.Use(middleware.JSONContentType)
	r.Use(middleware.Logger)

	if cfg.App.MetricsEnabled {
		r.Use(middleware.MetricsMiddleware)
	}

	r.NotFoundHandler = http.HandlerFunc(notFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(methodNotAllowed)

	registerRoutes(r, cfg, handlers)

	return r
}

func registerRoutes(r *mux.Router, cfg *config.Config, handlers Handlers) {
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods(http.MethodGet)

	if cfg.App.MetricsEnabled {
		r.Handle("/metrics", promhttp.Handler()).Methods(http.MethodGet)
	}
}

func notFound(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
}
