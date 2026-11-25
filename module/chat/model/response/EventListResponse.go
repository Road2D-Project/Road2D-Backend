package response

type EventListResponse struct {
	Events           []ConversationEventResponse `json:"events"`
	HasMore          bool                        `json:"hasMore"`
	NextEventSeq     int64                       `json:"nextEventSeq"`
	LatestEventSeq   int64                       `json:"latestEventSeq"`
	MinAfterEventSeq int64                       `json:"minAfterEventSeq"`
}
