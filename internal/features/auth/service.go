package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/infra/cache"
	"github.com/LuizFernando991/gym-api/internal/infra/email"
	"github.com/LuizFernando991/gym-api/internal/shared/entities"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxCodesPerHour    = 3
	rateLimitWindow    = time.Hour
	rateLimitKeyPrefix = "rate_limit:auth:verify_code:"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already in use")
	ErrSessionNotFound    = errors.New("session not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidCode        = errors.New("invalid or expired verification code")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrRateLimitExceeded  = errors.New("too many verification codes requested")
)

type Service interface {
	Register(ctx context.Context, req RegisterRequest) error
	VerifyRegistration(ctx context.Context, req VerifyEmailRequest) (*Session, error)
	RequestLogin(ctx context.Context, req LoginRequest) error
	VerifyLogin(ctx context.Context, req VerifyEmailRequest) (*Session, error)
	ResendRegistrationCode(ctx context.Context, email, ip string) error
	ResendLoginCode(ctx context.Context, email, ip string) error
	Me(ctx context.Context, userID string) (*User, error)
	Logout(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, userID string) ([]*Session, error)
	RevokeSession(ctx context.Context, userID, sessionID string) error
	ValidateToken(ctx context.Context, token string) (*entities.Session, error)
}

type service struct {
	repo        Repository
	cfg         config.AuthConfig
	emailSender email.Sender
	rateLimiter cache.RateLimiter
}

func NewService(repo Repository, cfg config.AuthConfig, emailSender email.Sender, rateLimiter cache.RateLimiter) Service {
	return &service{repo: repo, cfg: cfg, emailSender: emailSender, rateLimiter: rateLimiter}
}

func (s *service) Register(ctx context.Context, req RegisterRequest) error {
	_, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err == nil {
		return ErrEmailTaken
	}
	if !isNotFound(err) {
		return fmt.Errorf("auth: register: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("auth: hash password: %w", err)
	}

	if _, err := s.repo.CreateUser(ctx, req.Email, string(hash)); err != nil {
		return fmt.Errorf("auth: create user: %w", err)
	}

	return s.sendVerificationCode(ctx, req.Email, VerificationRegister, req.IPAddress)
}

func (s *service) VerifyRegistration(ctx context.Context, req VerifyEmailRequest) (*Session, error) {
	v, err := s.repo.FindEmailVerification(ctx, req.Email, req.Code, VerificationRegister)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrInvalidCode
		}
		return nil, fmt.Errorf("auth: verify registration: %w", err)
	}

	if err := s.repo.DeleteEmailVerification(ctx, v.ID); err != nil {
		return nil, fmt.Errorf("auth: delete verification: %w", err)
	}

	if err := s.repo.MarkUserVerified(ctx, req.Email); err != nil {
		return nil, fmt.Errorf("auth: mark verified: %w", err)
	}

	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("auth: find user: %w", err)
	}

	return s.createSession(ctx, user.ID, req.UserAgent, req.IPAddress)
}

func (s *service) RequestLogin(ctx context.Context, req LoginRequest) error {
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		if isNotFound(err) {
			return ErrInvalidCredentials
		}
		return fmt.Errorf("auth: login: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return ErrInvalidCredentials
	}

	if user.VerifiedAt == nil {
		return ErrEmailNotVerified
	}

	return s.sendVerificationCode(ctx, req.Email, VerificationLogin, req.IPAddress)
}

func (s *service) VerifyLogin(ctx context.Context, req VerifyEmailRequest) (*Session, error) {
	v, err := s.repo.FindEmailVerification(ctx, req.Email, req.Code, VerificationLogin)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrInvalidCode
		}
		return nil, fmt.Errorf("auth: verify login: %w", err)
	}

	if err := s.repo.DeleteEmailVerification(ctx, v.ID); err != nil {
		return nil, fmt.Errorf("auth: delete verification: %w", err)
	}

	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("auth: find user for login: %w", err)
	}

	return s.createSession(ctx, user.ID, req.UserAgent, req.IPAddress)
}

func (s *service) ResendRegistrationCode(ctx context.Context, emailAddr, ip string) error {
	user, err := s.repo.FindUserByEmail(ctx, emailAddr)
	if err != nil {
		if isNotFound(err) {
			return nil // don't reveal whether the email is registered
		}
		return fmt.Errorf("auth: resend registration: %w", err)
	}
	if user.VerifiedAt != nil {
		return nil // already verified; silently ignore
	}
	return s.sendVerificationCode(ctx, emailAddr, VerificationRegister, ip)
}

func (s *service) ResendLoginCode(ctx context.Context, emailAddr, ip string) error {
	user, err := s.repo.FindUserByEmail(ctx, emailAddr)
	if err != nil {
		if isNotFound(err) {
			return nil // don't reveal whether the email is registered
		}
		return fmt.Errorf("auth: resend login: %w", err)
	}
	if user.VerifiedAt == nil {
		return nil // unverified users cannot log in; silently ignore
	}
	return s.sendVerificationCode(ctx, emailAddr, VerificationLogin, ip)
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

func (s *service) sendVerificationCode(ctx context.Context, emailAddr string, vtype VerificationType, ip string) error {
	key := rateLimitKeyPrefix + ip + ":" + string(vtype)
	allowed, err := s.rateLimiter.Allow(ctx, key, maxCodesPerHour, rateLimitWindow)
	if err != nil {
		return fmt.Errorf("auth: rate limit check: %w", err)
	}
	if !allowed {
		return ErrRateLimitExceeded
	}

	if err := s.repo.DeleteEmailVerificationsByEmailAndType(ctx, emailAddr, vtype); err != nil {
		return fmt.Errorf("auth: cleanup old codes: %w", err)
	}

	code, err := generateCode()
	if err != nil {
		return fmt.Errorf("auth: generate code: %w", err)
	}

	expiresAt := time.Now().Add(15 * time.Minute)
	if _, err := s.repo.CreateEmailVerification(ctx, emailAddr, code, vtype, expiresAt); err != nil {
		return fmt.Errorf("auth: store code: %w", err)
	}

	html, err := email.VerificationCodeHTML(code)
	if err != nil {
		return fmt.Errorf("auth: render email: %w", err)
	}

	return s.emailSender.Send(ctx, email.Message{
		To:      emailAddr,
		Subject: "Your verification code",
		HTML:    html,
		Text:    code,
	})
}

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

func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
