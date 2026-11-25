package request

type ListConversationsRequest struct {
	Cursor string `form:"cursor"`
	Limit  int    `form:"limit"`
}
