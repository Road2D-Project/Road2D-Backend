package request

type MarkReadRequest struct {
	LastReadSeq *int64 `json:"lastReadSeq" binding:"required,gte=0"`
}
