package group

type CreateGroupRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

type JoinGroupRequest struct {
	InviteCode string `json:"invite_code" validate:"required"`
}

type SetReactionRequest struct {
	Emoji string `json:"emoji" validate:"required,min=1,max=16"`
}

type AddCommentRequest struct {
	Body string `json:"body" validate:"required,min=1,max=500"`
}
