package auth

import (
	"encoding/json"
	"errors"
	"net"
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
	public := r.PathPrefix("/auth").Subrouter()
	public.HandleFunc("/register", h.register).Methods(http.MethodPost)
	public.HandleFunc("/register/verify", h.verifyRegistration).Methods(http.MethodPost)
	public.HandleFunc("/register/resend", h.resendRegistrationCode).Methods(http.MethodPost)
	public.HandleFunc("/login", h.login).Methods(http.MethodPost)
	public.HandleFunc("/login/verify", h.verifyLogin).Methods(http.MethodPost)
	public.HandleFunc("/login/resend", h.resendLoginCode).Methods(http.MethodPost)

	private := r.PathPrefix("/auth").Subrouter()
	private.Use(authMiddleware)
	private.HandleFunc("/me", h.me).Methods(http.MethodGet)
	private.HandleFunc("/logout", h.logout).Methods(http.MethodPost)
	private.HandleFunc("/sessions", h.listSessions).Methods(http.MethodGet)
	private.HandleFunc("/sessions/{id}", h.revokeSession).Methods(http.MethodDelete)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	req.IPAddress = clientIP(r)

	err := h.svc.Register(r.Context(), req)
	if errors.Is(err, ErrEmailTaken) {
		httputil.Error(w, http.StatusConflict, "email already in use")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	httputil.JSON(w, http.StatusCreated, MessageResponse{Message: "verification code sent to your email"})
}

func (h *Handler) verifyRegistration(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	req.UserAgent = r.UserAgent()
	req.IPAddress = clientIP(r)

	session, err := h.svc.VerifyRegistration(r.Context(), req)
	if errors.Is(err, ErrInvalidCode) {
		httputil.Error(w, http.StatusUnprocessableEntity, "invalid or expired verification code")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	httputil.JSON(w, http.StatusOK, LoginResponse{Token: session.Token, ExpiresAt: session.ExpiresAt})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	req.IPAddress = clientIP(r)

	err := h.svc.RequestLogin(r.Context(), req)
	if errors.Is(err, ErrInvalidCredentials) {
		httputil.Error(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if errors.Is(err, ErrEmailNotVerified) {
		httputil.Error(w, http.StatusForbidden, "email not verified")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	httputil.JSON(w, http.StatusOK, MessageResponse{Message: "verification code sent to your email"})
}

func (h *Handler) verifyLogin(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	req.UserAgent = r.UserAgent()
	req.IPAddress = clientIP(r)

	session, err := h.svc.VerifyLogin(r.Context(), req)
	if errors.Is(err, ErrInvalidCode) {
		httputil.Error(w, http.StatusUnprocessableEntity, "invalid or expired verification code")
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

func (h *Handler) resendRegistrationCode(w http.ResponseWriter, r *http.Request) {
	var req ResendCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	err := h.svc.ResendRegistrationCode(r.Context(), req.Email, clientIP(r))
	if errors.Is(err, ErrRateLimitExceeded) {
		httputil.Error(w, http.StatusTooManyRequests, "too many requests, please try again later")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	httputil.JSON(w, http.StatusOK, MessageResponse{Message: "if eligible, a new verification code has been sent to your email"})
}

func (h *Handler) resendLoginCode(w http.ResponseWriter, r *http.Request) {
	var req ResendCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	err := h.svc.ResendLoginCode(r.Context(), req.Email, clientIP(r))
	if errors.Is(err, ErrRateLimitExceeded) {
		httputil.Error(w, http.StatusTooManyRequests, "too many requests, please try again later")
		return
	}
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	httputil.JSON(w, http.StatusOK, MessageResponse{Message: "if eligible, a new verification code has been sent to your email"})
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
