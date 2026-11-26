package response

type MessageListResponse struct {
	Messages      []MessageResponse `json:"messages"`
	HasMore       bool              `json:"hasMore"`
	NextBeforeSeq *int64            `json:"nextBeforeSeq,omitempty"`
	NextAfterSeq  *int64            `json:"nextAfterSeq,omitempty"`
}
