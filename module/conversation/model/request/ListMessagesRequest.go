package request

type ListMessagesRequest struct {
	BeforeSeq *int64 `form:"beforeSeq"`
	AfterSeq  *int64 `form:"afterSeq"`
	Limit     int    `form:"limit"`
}
