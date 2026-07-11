package auth

import "time"

type User struct {
	ID         string
	Email      string
	Nickname   *string
	VerifiedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
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

// ProviderClaims is the identity a social provider (Google/Apple) asserts about
// a user, extracted from a verified ID token.
type ProviderClaims struct {
	Subject       string `json:"subject"` // stable per-provider user id (the token's "sub")
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
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
