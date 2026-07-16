package group

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

// maxImageBytes caps uploaded images (cover / workout photo).
const maxImageBytes = 5 << 20 // 5 MiB

// Upload rate limit: per-user cap to protect storage bandwidth/cost.
const (
	uploadRateLimit  = 10
	uploadRateWindow = time.Minute
)

// Anti-abuse: cap comments per user per session (anti-flood).
const (
	commentRateLimit  = 10
	commentRateWindow = time.Hour
)

type Handler struct {
	svc Service
	rl  httputil.RateAllower
}

func NewHandler(svc Service, rl httputil.RateAllower) *Handler {
	return &Handler{svc: svc, rl: rl}
}

func (h *Handler) RegisterRoutes(r *mux.Router, authMiddleware func(http.Handler) http.Handler) {
	api := r.NewRoute().Subrouter()
	api.Use(authMiddleware)

	api.HandleFunc("/groups", h.createGroup).Methods(http.MethodPost)
	api.HandleFunc("/groups", h.listGroups).Methods(http.MethodGet)
	api.HandleFunc("/groups/join", h.joinGroup).Methods(http.MethodPost)
	api.HandleFunc("/groups/{id}", h.getGroup).Methods(http.MethodGet)
	api.HandleFunc("/groups/{id}/leave", h.leaveGroup).Methods(http.MethodDelete)
	api.HandleFunc("/groups/{id}/ranking", h.ranking).Methods(http.MethodGet)
	api.HandleFunc("/groups/{id}/feed", h.feed).Methods(http.MethodGet)
	api.HandleFunc("/groups/{id}/cover", h.setCover).Methods(http.MethodPatch)

	api.HandleFunc("/groups/{id}/sessions/{sessionId}", h.sessionDetail).Methods(http.MethodGet)
	api.HandleFunc("/groups/{id}/sessions/{sessionId}/reaction", h.setReaction).Methods(http.MethodPut)
	api.HandleFunc("/groups/{id}/sessions/{sessionId}/reaction", h.removeReaction).Methods(http.MethodDelete)
	api.HandleFunc("/groups/{id}/sessions/{sessionId}/comments", h.listComments).Methods(http.MethodGet)
	api.HandleFunc("/groups/{id}/sessions/{sessionId}/comments", h.addComment).Methods(http.MethodPost)
	api.HandleFunc("/groups/{id}/sessions/{sessionId}/comments/{commentId}", h.deleteComment).Methods(http.MethodDelete)
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	g, err := h.svc.CreateGroup(r.Context(), userID, req)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusCreated, g)
}

func (h *Handler) joinGroup(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	var req JoinGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	g, err := h.svc.JoinGroup(r.Context(), userID, req)
	if errors.Is(err, ErrInvalidInvite) {
		httputil.Error(w, http.StatusNotFound, "invalid invite code")
		return
	}
	if errors.Is(err, ErrAlreadyMember) {
		httputil.Error(w, http.StatusConflict, "already a member")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, g)
}

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	groups, err := h.svc.ListGroups(r.Context(), userID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, groups)
}

func (h *Handler) getGroup(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	id := mux.Vars(r)["id"]
	detail, err := h.svc.GetGroupDetail(r.Context(), id, userID)
	if h.writeGroupErr(w, err) {
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *Handler) leaveGroup(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	id := mux.Vars(r)["id"]
	err := h.svc.LeaveGroup(r.Context(), id, userID)
	if h.writeGroupErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ranking(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	id := mux.Vars(r)["id"]
	entries, err := h.svc.Ranking(r.Context(), id, userID)
	if h.writeGroupErr(w, err) {
		return
	}
	httputil.JSON(w, http.StatusOK, entries)
}

func (h *Handler) feed(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	id := mux.Vars(r)["id"]
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	page, err := h.svc.Feed(r.Context(), id, userID, q.Get("cursor"), limit)
	if errors.Is(err, ErrInvalidCursor) {
		httputil.Error(w, http.StatusBadRequest, "invalid cursor")
		return
	}
	if h.writeGroupErr(w, err) {
		return
	}
	httputil.JSON(w, http.StatusOK, page)
}

func (h *Handler) setCover(w http.ResponseWriter, r *http.Request) {
	if !httputil.EnforceUserRateLimit(w, r, h.rl, "upload", uploadRateLimit, uploadRateWindow) {
		return
	}
	userID := httputil.SessionFromContext(r.Context()).UserID
	id := mux.Vars(r)["id"]

	file, contentType, ok := httputil.ReadImageUpload(w, r, maxImageBytes)
	if !ok {
		return
	}
	defer file.Close()

	g, err := h.svc.SetCover(r.Context(), id, userID, contentType, file)
	if errors.Is(err, ErrUnsupportedImage) {
		httputil.Error(w, http.StatusBadRequest, "unsupported image type (use jpeg or png)")
		return
	}
	if h.writeGroupErr(w, err) {
		return
	}
	httputil.JSON(w, http.StatusOK, g)
}

func (h *Handler) sessionDetail(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	v := mux.Vars(r)
	detail, err := h.svc.SessionDetail(r.Context(), v["id"], v["sessionId"], userID)
	if h.writeGroupErr(w, err) {
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *Handler) setReaction(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	v := mux.Vars(r)
	var req SetReactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	detail, err := h.svc.SetReaction(r.Context(), v["id"], v["sessionId"], userID, req.Emoji)
	if h.writeGroupErr(w, err) {
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *Handler) removeReaction(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	v := mux.Vars(r)
	detail, err := h.svc.RemoveReaction(r.Context(), v["id"], v["sessionId"], userID)
	if h.writeGroupErr(w, err) {
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *Handler) listComments(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	v := mux.Vars(r)
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	page, err := h.svc.Comments(r.Context(), v["id"], v["sessionId"], userID, q.Get("cursor"), limit)
	if errors.Is(err, ErrInvalidCursor) {
		httputil.Error(w, http.StatusBadRequest, "invalid cursor")
		return
	}
	if h.writeGroupErr(w, err) {
		return
	}
	httputil.JSON(w, http.StatusOK, page)
}

func (h *Handler) addComment(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	v := mux.Vars(r)
	if !httputil.EnforceRateLimit(w, r, h.rl, "comment:"+v["sessionId"]+":"+userID, commentRateLimit, commentRateWindow) {
		return
	}
	var req AddCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	comment, err := h.svc.AddComment(r.Context(), v["id"], v["sessionId"], userID, req)
	if h.writeGroupErr(w, err) {
		return
	}
	httputil.JSON(w, http.StatusCreated, comment)
}

func (h *Handler) deleteComment(w http.ResponseWriter, r *http.Request) {
	userID := httputil.SessionFromContext(r.Context()).UserID
	v := mux.Vars(r)
	err := h.svc.DeleteComment(r.Context(), v["id"], v["sessionId"], v["commentId"], userID)
	if h.writeGroupErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeGroupErr maps common group errors to responses. Returns true if it wrote one.
func (h *Handler) writeGroupErr(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrNotFound):
		httputil.Error(w, http.StatusNotFound, "group not found")
	case errors.Is(err, ErrForbidden):
		httputil.Error(w, http.StatusForbidden, "access denied")
	default:
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
	}
	return true
}
