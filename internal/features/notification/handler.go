package notification

import (
	"encoding/json"
	"net/http"

	"github.com/LuizFernando991/gym-api/internal/shared/httputil"
	"github.com/LuizFernando991/gym-api/internal/shared/validate"
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

	api.HandleFunc("/me/push-token", h.registerToken).Methods(http.MethodPost)
	api.HandleFunc("/me/push-token", h.deleteToken).Methods(http.MethodDelete)
}

func (h *Handler) registerToken(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	var req RegisterTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.RegisterToken(r.Context(), userID, req.Token, req.Platform); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteToken(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	var req DeleteTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.DeleteToken(r.Context(), userID, req.Token); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
