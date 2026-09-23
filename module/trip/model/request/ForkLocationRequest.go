package request

import "time"

type ForkLocationRequest struct {
	Name         string     `json:"name"`
	ArriveTime   *time.Time `json:"arriveTime,omitempty"`
	StayOverTime int        `json:"stayOverTime"`
}
