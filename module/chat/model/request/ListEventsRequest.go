package request

type ListEventsRequest struct {
	AfterEventSeq int64 `form:"afterEventSeq"`
	Limit         int   `form:"limit"`
}
