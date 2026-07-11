package notification

type RegisterTokenRequest struct {
	Token    string `json:"token"    validate:"required"`
	Platform string `json:"platform" validate:"omitempty,oneof=ios android web"`
}

type DeleteTokenRequest struct {
	Token string `json:"token" validate:"required"`
}
