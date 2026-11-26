package response

type ConversationListResponse struct {
	Conversations []ConversationResponse `json:"conversations"`
	NextCursor    string                 `json:"nextCursor,omitempty"`
}
