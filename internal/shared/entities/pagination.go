package entities

type CursorPage[T any] struct {
	Data   []T        `json:"data"`
	Cursor CursorMeta `json:"cursor"`
}

type CursorMeta struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}
