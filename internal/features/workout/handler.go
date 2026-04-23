package workout

import (
	"encoding/json"
	"errors"
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

	api.HandleFunc("/exercises", h.listExercises).Methods(http.MethodGet)
	api.HandleFunc("/exercises", h.createExercise).Methods(http.MethodPost)
	api.HandleFunc("/exercises/{id}", h.getExercise).Methods(http.MethodGet)

	api.HandleFunc("/workouts", h.listWorkouts).Methods(http.MethodGet)
	api.HandleFunc("/workouts", h.createWorkout).Methods(http.MethodPost)
	api.HandleFunc("/workouts/{id}", h.getWorkout).Methods(http.MethodGet)
	api.HandleFunc("/workouts/{id}", h.updateWorkout).Methods(http.MethodPut)
	api.HandleFunc("/workouts/{id}", h.deleteWorkout).Methods(http.MethodDelete)
	api.HandleFunc("/workouts/{id}/exercises", h.addWorkoutExercise).Methods(http.MethodPost)
	api.HandleFunc("/workouts/{id}/exercises/reorder", h.reorderWorkoutExercises).Methods(http.MethodPatch)
	api.HandleFunc("/workouts/{id}/exercises/{weId}", h.updateWorkoutExercise).Methods(http.MethodPut)
	api.HandleFunc("/workouts/{id}/exercises/{weId}", h.deleteWorkoutExercise).Methods(http.MethodDelete)
	api.HandleFunc("/workouts/{id}/progress", h.getWorkoutProgress).Methods(http.MethodGet)

	// /workout-sessions/missed must be registered before /{id} to avoid variable capture.
	api.HandleFunc("/workout-sessions/missed", h.getMissedSessions).Methods(http.MethodGet)
	api.HandleFunc("/workout-sessions", h.listSessions).Methods(http.MethodGet)
	api.HandleFunc("/workout-sessions", h.createSession).Methods(http.MethodPost)
	api.HandleFunc("/workout-sessions/{id}", h.getSession).Methods(http.MethodGet)
	api.HandleFunc("/workout-sessions/{id}", h.updateSession).Methods(http.MethodPut)
	api.HandleFunc("/workout-sessions/{id}/sets", h.recordSet).Methods(http.MethodPost)
	api.HandleFunc("/workout-sessions/{id}/sets/{setId}", h.updateSet).Methods(http.MethodPut)
	api.HandleFunc("/workout-sessions/{id}/sets/{setId}", h.deleteSet).Methods(http.MethodDelete)
}

// ── Exercises ──────────────────────────────────────────────────────────────────

func (h *Handler) listExercises(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	search := q.Get("search")
	cursor := q.Get("cursor")

	limit := 20
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			httputil.Error(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
		limit = n
	}

	page, err := h.svc.ListExercises(r.Context(), search, cursor, limit)
	if errors.Is(err, ErrInvalidCursor) {
		httputil.Error(w, http.StatusBadRequest, "invalid cursor")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, page)
}

func (h *Handler) createExercise(w http.ResponseWriter, r *http.Request) {
	var req CreateExerciseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	e, err := h.svc.CreateExercise(r.Context(), req)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusCreated, e)
}

func (h *Handler) getExercise(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	e, err := h.svc.GetExercise(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "exercise not found")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, e)
}

// ── Workouts ───────────────────────────────────────────────────────────────────

func (h *Handler) listWorkouts(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	workouts, err := h.svc.ListWorkouts(r.Context(), userID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if workouts == nil {
		workouts = []WorkoutDetail{}
	}
	httputil.JSON(w, http.StatusOK, workouts)
}

func (h *Handler) createWorkout(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	var req CreateWorkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	workout, err := h.svc.CreateWorkout(r.Context(), userID, req)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusCreated, workout)
}

func (h *Handler) getWorkout(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	id := mux.Vars(r)["id"]
	detail, err := h.svc.GetWorkout(r.Context(), id, userID)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "workout not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *Handler) updateWorkout(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	id := mux.Vars(r)["id"]
	var req UpdateWorkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	workout, err := h.svc.UpdateWorkout(r.Context(), id, userID, req)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "workout not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, workout)
}

func (h *Handler) deleteWorkout(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	id := mux.Vars(r)["id"]
	err := h.svc.DeleteWorkout(r.Context(), id, userID)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "workout not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── WorkoutExercises ───────────────────────────────────────────────────────────

func (h *Handler) addWorkoutExercise(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	workoutID := mux.Vars(r)["id"]
	var req AddWorkoutExerciseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	we, err := h.svc.AddWorkoutExercise(r.Context(), workoutID, userID, req)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "workout not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if errors.Is(err, ErrWorkoutExerciseLimit) {
		httputil.Error(w, http.StatusBadRequest, "workout cannot have more than 50 exercises")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusCreated, we)
}

func (h *Handler) reorderWorkoutExercises(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	workoutID := mux.Vars(r)["id"]
	var req ReorderWorkoutExercisesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	err := h.svc.ReorderWorkoutExercises(r.Context(), workoutID, userID, req)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "workout exercise not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if errors.Is(err, ErrInvalidExerciseReordering) {
		httputil.Error(w, http.StatusBadRequest, "exercise ids must be unique")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) updateWorkoutExercise(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	vars := mux.Vars(r)
	workoutID, weID := vars["id"], vars["weId"]
	var req UpdateWorkoutExerciseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	we, err := h.svc.UpdateWorkoutExercise(r.Context(), weID, workoutID, userID, req)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "workout exercise not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, we)
}

func (h *Handler) deleteWorkoutExercise(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	vars := mux.Vars(r)
	workoutID, weID := vars["id"], vars["weId"]
	err := h.svc.DeleteWorkoutExercise(r.Context(), weID, workoutID, userID)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "workout exercise not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getWorkoutProgress(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	workoutID := mux.Vars(r)["id"]

	var exerciseID *string
	if v := r.URL.Query().Get("exercise_id"); v != "" {
		exerciseID = &v
	}

	progress, err := h.svc.GetWorkoutProgress(r.Context(), workoutID, userID, exerciseID)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "workout not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if progress == nil {
		progress = []ExerciseProgress{}
	}
	httputil.JSON(w, http.StatusOK, progress)
}

// ── Sessions ───────────────────────────────────────────────────────────────────

func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	q := r.URL.Query()

	var workoutID *string
	if v := q.Get("workout_id"); v != "" {
		workoutID = &v
	}

	var from, to *time.Time
	if v := q.Get("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid from date, use YYYY-MM-DD")
			return
		}
		from = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid to date, use YYYY-MM-DD")
			return
		}
		to = &t
	}

	sessions, err := h.svc.ListSessions(r.Context(), userID, workoutID, from, to)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if sessions == nil {
		sessions = []WorkoutSession{}
	}
	httputil.JSON(w, http.StatusOK, sessions)
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	var req CreateWorkoutSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	sess, err := h.svc.CreateSession(r.Context(), userID, req)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "workout not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusCreated, sess)
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	id := mux.Vars(r)["id"]
	detail, err := h.svc.GetSession(r.Context(), id, userID)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "session not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *Handler) updateSession(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	id := mux.Vars(r)["id"]
	var req UpdateWorkoutSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	sess, err := h.svc.UpdateSession(r.Context(), id, userID, req)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "session not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, sess)
}

func (h *Handler) getMissedSessions(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	q := r.URL.Query()

	now := time.Now()
	from := now.AddDate(0, -1, 0)
	to := now

	if v := q.Get("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid from date, use YYYY-MM-DD")
			return
		}
		from = t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid to date, use YYYY-MM-DD")
			return
		}
		to = t
	}

	missed, err := h.svc.GetMissedSessions(r.Context(), userID, from, to)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if missed == nil {
		missed = []MissedSession{}
	}
	httputil.JSON(w, http.StatusOK, missed)
}

// ── Sets ───────────────────────────────────────────────────────────────────────

func (h *Handler) recordSet(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	sessionID := mux.Vars(r)["id"]
	var req RecordSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	set, err := h.svc.RecordSet(r.Context(), sessionID, userID, req)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "session not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusCreated, set)
}

func (h *Handler) updateSet(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	vars := mux.Vars(r)
	sessionID, setID := vars["id"], vars["setId"]
	var req UpdateSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	set, err := h.svc.UpdateSet(r.Context(), setID, sessionID, userID, req)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "set not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, set)
}

func (h *Handler) deleteSet(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	vars := mux.Vars(r)
	sessionID, setID := vars["id"], vars["setId"]
	err := h.svc.DeleteSet(r.Context(), setID, sessionID, userID)
	if errors.Is(err, ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "set not found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
