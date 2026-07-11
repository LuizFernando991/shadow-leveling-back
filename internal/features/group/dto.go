package group

type CreateGroupRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

type JoinGroupRequest struct {
	InviteCode string `json:"invite_code" validate:"required"`
}
