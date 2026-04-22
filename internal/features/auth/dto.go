package auth

import "time"

type RegisterRequest struct {
	Email     string `json:"email"    validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	IPAddress string `json:"-"`
}

type LoginRequest struct {
	Email     string `json:"email"    validate:"required,email"`
	Password  string `json:"password" validate:"required"`
	IPAddress string `json:"-"`
}

type VerifyEmailRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Code      string `json:"code"  validate:"required,len=6,numeric"`
	UserAgent string `json:"-"`
	IPAddress string `json:"-"`
}

type ResendCodeRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type LoginResponse struct {
	Token     string     `json:"token"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type SessionResponse struct {
	ID        string     `json:"id"`
	UserAgent string     `json:"user_agent"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}
