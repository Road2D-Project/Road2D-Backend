package response

import "Road-To-Destination-BE/module/share"

type ReplayRequiredResponse struct {
	share.ErrorResponse
	SyncRequired     bool  `json:"syncRequired"`
	MinAfterEventSeq int64 `json:"minAfterEventSeq"`
	LatestEventSeq   int64 `json:"latestEventSeq"`
}
