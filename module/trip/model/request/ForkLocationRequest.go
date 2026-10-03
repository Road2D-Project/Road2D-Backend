package request

import "time"

type ForkLocationRequest struct {
	Name         string     `json:"name"`
	ArriveTime   *time.Time `json:"arriveTime,omitempty" binding:"omitempty,arriveTime"`
	StayOverTime int        `json:"stayOverTime" binding:"stayMinutes"`
}
