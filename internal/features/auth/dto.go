package auth

import "time"

type EmailCodeRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type SocialLoginRequest struct {
	Provider  string `json:"provider" validate:"required,oneof=google apple"`
	IDToken   string `json:"id_token" validate:"required"`
	UserAgent string `json:"-"`
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
	Nickname  *string   `json:"nickname"`
	CreatedAt time.Time `json:"created_at"`
}

type UpdateProfileRequest struct {
	Nickname string `json:"nickname" validate:"required,min=2,max=30"`
}
