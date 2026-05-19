package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/shared/entities"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already in use")
	ErrSessionNotFound    = errors.New("session not found")
	ErrUnauthorized       = errors.New("unauthorized")
)

type Service interface {
	Register(ctx context.Context, req RegisterRequest) (*Session, error)
	Login(ctx context.Context, req LoginRequest) (*Session, error)
	Me(ctx context.Context, userID string) (*User, error)
	UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*User, error)
	Logout(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, userID string) ([]*Session, error)
	RevokeSession(ctx context.Context, userID, sessionID string) error
	ValidateToken(ctx context.Context, token string) (*entities.Session, error)
}

type service struct {
	repo Repository
	cfg  config.AuthConfig
}

func NewService(repo Repository, cfg config.AuthConfig) Service {
	return &service{repo: repo, cfg: cfg}
}

func (s *service) Register(ctx context.Context, req RegisterRequest) (*Session, error) {
	_, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err == nil {
		return nil, ErrEmailTaken
	}
	if !isNotFound(err) {
		return nil, fmt.Errorf("auth: register: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("auth: hash password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, req.Email, string(hash))
	if err != nil {
		return nil, fmt.Errorf("auth: create user: %w", err)
	}

	if err := s.repo.MarkUserVerified(ctx, req.Email); err != nil {
		return nil, fmt.Errorf("auth: mark verified: %w", err)
	}

	return s.createSession(ctx, user.ID, req.UserAgent, req.IPAddress)
}

func (s *service) Login(ctx context.Context, req LoginRequest) (*Session, error) {
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("auth: login: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.createSession(ctx, user.ID, req.UserAgent, req.IPAddress)
}

func (s *service) Me(ctx context.Context, userID string) (*User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("auth: me: %w", err)
	}
	return user, nil
}

func (s *service) UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*User, error) {
	user, err := s.repo.UpdateNickname(ctx, userID, req.Nickname)
	if err != nil {
		return nil, fmt.Errorf("auth: update profile: %w", err)
	}
	return user, nil
}

func (s *service) Logout(ctx context.Context, sessionID string) error {
	return s.repo.RevokeSession(ctx, sessionID)
}

func (s *service) ListSessions(ctx context.Context, userID string) ([]*Session, error) {
	return s.repo.ListSessionsByUserID(ctx, userID)
}

func (s *service) RevokeSession(ctx context.Context, userID, sessionID string) error {
	session, err := s.repo.FindSessionByID(ctx, sessionID)
	if err != nil {
		if isNotFound(err) {
			return ErrSessionNotFound
		}
		return fmt.Errorf("auth: revoke session: %w", err)
	}
	if session.UserID != userID {
		return ErrUnauthorized
	}
	return s.repo.RevokeSession(ctx, sessionID)
}

func (s *service) ValidateToken(ctx context.Context, token string) (*entities.Session, error) {
	session, err := s.repo.FindSessionByToken(ctx, token)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("auth: validate token: %w", err)
	}
	if !session.IsValid() {
		return nil, ErrUnauthorized
	}
	return &entities.Session{ID: session.ID, UserID: session.UserID}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (s *service) createSession(ctx context.Context, userID, userAgent, ipAddress string) (*Session, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("auth: generate token: %w", err)
	}
	expiresAt := time.Now().Add(s.cfg.TokenTTL)
	return s.repo.CreateSession(ctx, userID, token, userAgent, ipAddress, &expiresAt)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}


func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
