package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/infra/email"
	"github.com/LuizFernando991/gym-api/internal/infra/storage"
	"github.com/LuizFernando991/gym-api/internal/shared/entities"
)

// codeTTL is how long an email verification code stays valid. Matches the
// "expires in 15 minutes" copy in the verification-code email template.
const codeTTL = 15 * time.Minute

var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrInvalidCode      = errors.New("invalid or expired code")
	ErrInvalidToken     = errors.New("invalid provider token")
	ErrUnsupportedImage = errors.New("unsupported image type")
)

// TokenVerifier verifies a social provider's ID token and returns the identity
// it asserts. Implemented by internal/infra/oidc in production; faked in tests.
type TokenVerifier interface {
	Verify(ctx context.Context, provider, idToken string) (*ProviderClaims, error)
}

type Service interface {
	RequestEmailCode(ctx context.Context, addr string) error
	VerifyEmailCode(ctx context.Context, req VerifyEmailRequest) (*Session, error)
	SocialLogin(ctx context.Context, req SocialLoginRequest) (*Session, error)
	Me(ctx context.Context, userID string) (*User, error)
	UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*User, error)
	UpdateAvatar(ctx context.Context, userID, contentType string, r io.Reader) (*User, error)
	Logout(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, userID string) ([]*Session, error)
	RevokeSession(ctx context.Context, userID, sessionID string) error
	ValidateToken(ctx context.Context, token string) (*entities.Session, error)
}

type service struct {
	repo     Repository
	cfg      config.AuthConfig
	sender   email.Sender
	verifier TokenVerifier
	uploader storage.Uploader
}

func NewService(repo Repository, cfg config.AuthConfig, sender email.Sender, verifier TokenVerifier, uploader storage.Uploader) Service {
	return &service{repo: repo, cfg: cfg, sender: sender, verifier: verifier, uploader: uploader}
}

// RequestEmailCode generates a fresh 6-digit code for addr, replacing any
// prior code, and emails it. Used by the passwordless email flow; it never
// reveals whether the address already has an account (create-or-login happens
// at verify time).
func (s *service) RequestEmailCode(ctx context.Context, addr string) error {
	code, err := generateCode()
	if err != nil {
		return fmt.Errorf("auth: generate code: %w", err)
	}

	if err := s.repo.DeleteEmailVerificationsByEmailAndType(ctx, addr, VerificationLogin); err != nil {
		return fmt.Errorf("auth: clear old codes: %w", err)
	}
	if _, err := s.repo.CreateEmailVerification(ctx, addr, code, VerificationLogin, time.Now().Add(codeTTL)); err != nil {
		return fmt.Errorf("auth: create code: %w", err)
	}

	html, err := email.VerificationCodeHTML(code)
	if err != nil {
		return fmt.Errorf("auth: render code email: %w", err)
	}
	return s.sender.Send(ctx, email.Message{
		To:      addr,
		Subject: "Your verification code",
		HTML:    html,
		Text:    code,
	})
}

// VerifyEmailCode validates a code and issues a session. Unified flow: if the
// email has no account yet, one is created (verified); otherwise it logs in.
// The code is single-use.
func (s *service) VerifyEmailCode(ctx context.Context, req VerifyEmailRequest) (*Session, error) {
	v, err := s.repo.FindEmailVerification(ctx, req.Email, req.Code, VerificationLogin)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrInvalidCode
		}
		return nil, fmt.Errorf("auth: find code: %w", err)
	}

	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		if !isNotFound(err) {
			return nil, fmt.Errorf("auth: verify code: %w", err)
		}
		user, err = s.repo.CreateUser(ctx, req.Email)
		if err != nil {
			return nil, fmt.Errorf("auth: create user: %w", err)
		}
		if err := s.repo.MarkUserVerified(ctx, req.Email); err != nil {
			return nil, fmt.Errorf("auth: mark verified: %w", err)
		}
	}

	if err := s.repo.DeleteEmailVerification(ctx, v.ID); err != nil {
		return nil, fmt.Errorf("auth: consume code: %w", err)
	}

	return s.createSession(ctx, user.ID, req.UserAgent, req.IPAddress)
}

// SocialLogin verifies a provider ID token and issues a session. Resolution
// order: (1) a known identity for that provider+subject logs its user in;
// (2) otherwise, if the provider asserts a verified email that matches an
// existing user, the new identity is linked to that user; (3) otherwise a new
// verified user is created. Steps 2-3 also record the identity for next time.
func (s *service) SocialLogin(ctx context.Context, req SocialLoginRequest) (*Session, error) {
	claims, err := s.verifier.Verify(ctx, req.Provider, req.IDToken)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Subject == "" {
		return nil, ErrInvalidToken
	}

	userID, err := s.repo.FindUserIDByProviderSubject(ctx, req.Provider, claims.Subject)
	if err != nil && !isNotFound(err) {
		return nil, fmt.Errorf("auth: find identity: %w", err)
	}

	if userID == "" {
		userID, err = s.resolveUserForNewIdentity(ctx, claims)
		if err != nil {
			return nil, err
		}
		if err := s.repo.CreateIdentity(ctx, userID, req.Provider, claims.Subject); err != nil {
			return nil, fmt.Errorf("auth: create identity: %w", err)
		}
	}

	return s.createSession(ctx, userID, req.UserAgent, req.IPAddress)
}

// resolveUserForNewIdentity links a first-time identity to an existing user
// when the provider asserts a verified matching email, otherwise creates one.
func (s *service) resolveUserForNewIdentity(ctx context.Context, claims *ProviderClaims) (string, error) {
	if claims.EmailVerified && claims.Email != "" {
		user, err := s.repo.FindUserByEmail(ctx, claims.Email)
		if err == nil {
			return user.ID, nil
		}
		if !isNotFound(err) {
			return "", fmt.Errorf("auth: find user by email: %w", err)
		}
	}

	if claims.Email == "" {
		// No email to key a new account on (e.g. a hidden Apple relay that was
		// not shared). Nothing to link or create against.
		return "", ErrInvalidToken
	}
	user, err := s.repo.CreateUser(ctx, claims.Email)
	if err != nil {
		return "", fmt.Errorf("auth: create user: %w", err)
	}
	if err := s.repo.MarkUserVerified(ctx, claims.Email); err != nil {
		return "", fmt.Errorf("auth: mark verified: %w", err)
	}
	return user.ID, nil
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

// UpdateAvatar stores the image in the bucket under the user's id (overwriting
// any previous avatar) and saves the resulting URL on the user.
func (s *service) UpdateAvatar(ctx context.Context, userID, contentType string, r io.Reader) (*User, error) {
	ext, ok := storage.ExtForContentType(contentType)
	if !ok {
		return nil, ErrUnsupportedImage
	}
	url, err := s.uploader.Upload(ctx, "avatars/"+userID+ext, contentType, r)
	if err != nil {
		return nil, fmt.Errorf("auth: upload avatar: %w", err)
	}
	user, err := s.repo.UpdateAvatarURL(ctx, userID, url)
	if err != nil {
		return nil, fmt.Errorf("auth: update avatar: %w", err)
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

// generateCode returns a cryptographically-random 6-digit numeric code,
// zero-padded, matching the CHAR(6) column and the len=6 validation.
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
