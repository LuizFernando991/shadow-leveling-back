package usermetrics

import (
	"net/http"

	"github.com/LuizFernando991/gym-api/internal/shared/httputil"
	"github.com/gorilla/mux"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r *mux.Router, authMiddleware func(http.Handler) http.Handler) {
	api := r.NewRoute().Subrouter()
	api.Use(authMiddleware)

	api.HandleFunc("/user-metrics/today", h.getTodayMissions).Methods(http.MethodGet)
}

func (h *Handler) getTodayMissions(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	resp, err := h.svc.GetTodayMissions(r.Context(), userID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}
