package request

type UpdateMessageRequest struct {
	Text             string `json:"text" binding:"required,max=4000"`
	ExpectedRevision int64  `json:"expectedRevision" binding:"required,gte=1"`
}
