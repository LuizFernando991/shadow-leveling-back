package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/LuizFernando991/gym-api/internal/shared/httputil"
	"github.com/LuizFernando991/gym-api/internal/shared/validate"
	"github.com/gorilla/mux"
)

// Rate limits for the unauthenticated passwordless email flow.
const (
	codeSendLimit       = 3
	codeSendWindow      = time.Minute
	codeSendHourlyLimit = 10
	codeVerifyLimit     = 5
	codeVerifyWindow    = 10 * time.Minute
)

type Handler struct {
	svc     Service
	limiter httputil.RateAllower
}

func NewHandler(svc Service, limiter httputil.RateAllower) *Handler {
	return &Handler{svc: svc, limiter: limiter}
}

func (h *Handler) RegisterRoutes(r *mux.Router, authMiddleware func(http.Handler) http.Handler) {
	public := r.PathPrefix("/auth").Subrouter()
	public.HandleFunc("/email/request", h.requestEmailCode).Methods(http.MethodPost)
	public.HandleFunc("/email/verify", h.verifyEmailCode).Methods(http.MethodPost)
	public.HandleFunc("/social", h.socialLogin).Methods(http.MethodPost)

	private := r.PathPrefix("/auth").Subrouter()
	private.Use(authMiddleware)
	private.HandleFunc("/me", h.me).Methods(http.MethodGet)
	private.HandleFunc("/me", h.updateProfile).Methods(http.MethodPatch)
	private.HandleFunc("/logout", h.logout).Methods(http.MethodPost)
	private.HandleFunc("/sessions", h.listSessions).Methods(http.MethodGet)
	private.HandleFunc("/sessions/{id}", h.revokeSession).Methods(http.MethodDelete)
}

func (h *Handler) requestEmailCode(w http.ResponseWriter, r *http.Request) {
	var req EmailCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	// Short burst throttle per email, plus an hourly cap per email+IP so a
	// single sender can't bomb an inbox over a longer window.
	if !httputil.EnforceRateLimit(w, r, h.limiter, "email-code:"+req.Email, codeSendLimit, codeSendWindow) {
		return
	}
	if !httputil.EnforceRateLimit(w, r, h.limiter, "email-code-hourly:"+req.Email+":"+clientIP(r), codeSendHourlyLimit, time.Hour) {
		return
	}

	if err := h.svc.RequestEmailCode(r.Context(), req.Email); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, MessageResponse{Message: "verification code sent"})
}

func (h *Handler) verifyEmailCode(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	// ponytail: cap verify attempts per email via the limiter instead of a
	// per-code attempt counter. 5 tries / 10min against a 1e6 code space makes
	// brute force negligible; add a per-code counter only if that ever matters.
	if !httputil.EnforceRateLimit(w, r, h.limiter, "email-verify:"+req.Email, codeVerifyLimit, codeVerifyWindow) {
		return
	}
	req.IPAddress = clientIP(r)
	req.UserAgent = r.UserAgent()

	session, err := h.svc.VerifyEmailCode(r.Context(), req)
	if errors.Is(err, ErrInvalidCode) {
		httputil.Error(w, http.StatusUnprocessableEntity, "invalid or expired code")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, LoginResponse{Token: session.Token, ExpiresAt: session.ExpiresAt})
}

func (h *Handler) socialLogin(w http.ResponseWriter, r *http.Request) {
	var req SocialLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	req.IPAddress = clientIP(r)
	req.UserAgent = r.UserAgent()

	session, err := h.svc.SocialLogin(r.Context(), req)
	if errors.Is(err, ErrInvalidToken) {
		httputil.Error(w, http.StatusUnauthorized, "invalid provider token")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, LoginResponse{Token: session.Token, ExpiresAt: session.ExpiresAt})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	session := httputil.SessionFromContext(r.Context())
	user, err := h.svc.Me(r.Context(), session.UserID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Nickname:  user.Nickname,
		CreatedAt: user.CreatedAt,
	})
}

func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	session := httputil.SessionFromContext(r.Context())
	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := h.svc.UpdateProfile(r.Context(), session.UserID, req)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.JSON(w, http.StatusOK, UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Nickname:  user.Nickname,
		CreatedAt: user.CreatedAt,
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	session := httputil.SessionFromContext(r.Context())
	if err := h.svc.Logout(r.Context(), session.ID); err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	session := httputil.SessionFromContext(r.Context())
	sessions, err := h.svc.ListSessions(r.Context(), session.UserID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]SessionResponse, len(sessions))
	for i, s := range sessions {
		resp[i] = SessionResponse{
			ID:        s.ID,
			UserAgent: s.UserAgent,
			CreatedAt: s.CreatedAt,
			ExpiresAt: s.ExpiresAt,
		}
	}
	httputil.JSON(w, http.StatusOK, resp)
}

func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	sessionID := mux.Vars(r)["id"]
	currentSession := httputil.SessionFromContext(r.Context())

	err := h.svc.RevokeSession(r.Context(), currentSession.UserID, sessionID)
	if errors.Is(err, ErrSessionNotFound) {
		httputil.Error(w, http.StatusNotFound, "session not found")
		return
	}
	if errors.Is(err, ErrUnauthorized) {
		httputil.Error(w, http.StatusForbidden, "forbidden")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}


func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
