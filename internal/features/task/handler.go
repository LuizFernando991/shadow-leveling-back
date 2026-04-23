package task

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

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

	api.HandleFunc("/tasks", h.createTask).Methods(http.MethodPost)
	api.HandleFunc("/tasks/uncompleted", h.listUncompletedByDay).Methods(http.MethodGet)
	api.HandleFunc("/tasks/month", h.listByMonth).Methods(http.MethodGet)
	api.HandleFunc("/tasks/day", h.listByDay).Methods(http.MethodGet)
	api.HandleFunc("/tasks/{id}/complete", h.completeTask).Methods(http.MethodPatch)
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	t, err := h.svc.CreateTask(r.Context(), userID, req)
	if errors.Is(err, ErrInvalidDate) {
		httputil.Error(w, http.StatusBadRequest, "final_date must be on or after initial_date")
		return
	}
	if errors.Is(err, ErrInvalidRequest) {
		httputil.Error(w, http.StatusBadRequest, "custom_days_of_week is required when recurrence_type is custom")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusCreated, t)
}

func (h *Handler) completeTask(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	id := mux.Vars(r)["id"]

	var req CompleteTaskRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			httputil.Error(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	occurrence, err := h.svc.CompleteTask(r.Context(), id, userID, req)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "task not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if errors.Is(err, ErrNotScheduled) {
		httputil.Error(w, http.StatusBadRequest, "task is not scheduled for date")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, occurrence)
}

func (h *Handler) listUncompletedByDay(w http.ResponseWriter, r *http.Request) {
	d, ok := parseDateQuery(w, r, "date")
	if !ok {
		return
	}
	userID := httputil.SessionFromContext(r.Context()).UserID
	tasks, err := h.svc.ListUncompletedByDay(r.Context(), userID, d)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, tasks)
}

func (h *Handler) listByDay(w http.ResponseWriter, r *http.Request) {
	d, ok := parseDateQuery(w, r, "date")
	if !ok {
		return
	}
	userID := httputil.SessionFromContext(r.Context()).UserID
	tasks, err := h.svc.ListByDay(r.Context(), userID, d)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, tasks)
}

func (h *Handler) listByMonth(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	year, err := strconv.Atoi(q.Get("year"))
	if err != nil || year < 1 {
		httputil.Error(w, http.StatusBadRequest, "invalid year")
		return
	}
	monthNum, err := strconv.Atoi(q.Get("month"))
	if err != nil || monthNum < 1 || monthNum > 12 {
		httputil.Error(w, http.StatusBadRequest, "invalid month")
		return
	}

	userID := httputil.SessionFromContext(r.Context()).UserID
	tasks, err := h.svc.ListByMonth(r.Context(), userID, year, time.Month(monthNum))
	if errors.Is(err, ErrInvalidDate) {
		httputil.Error(w, http.StatusBadRequest, "invalid month")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, tasks)
}

func parseDateQuery(w http.ResponseWriter, r *http.Request, name string) (time.Time, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		httputil.Error(w, http.StatusBadRequest, name+" is required")
		return time.Time{}, false
	}
	d, err := time.Parse("2006-01-02", value)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid "+name+", use YYYY-MM-DD")
		return time.Time{}, false
	}
	return d, true
}
