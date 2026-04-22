package auth

import "time"

type User struct {
	ID           string
	Email        string
	PasswordHash string
	VerifiedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID        string
	UserID    string
	Token     string
	UserAgent string
	IPAddress string
	ExpiresAt *time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (s *Session) IsValid() bool {
	if s.RevokedAt != nil {
		return false
	}
	return s.ExpiresAt == nil || s.ExpiresAt.After(time.Now())
}

type VerificationType string

const (
	VerificationRegister VerificationType = "register"
	VerificationLogin    VerificationType = "login"
)

type EmailVerification struct {
	ID        string
	Email     string
	Code      string
	Type      VerificationType
	ExpiresAt time.Time
	CreatedAt time.Time
}
