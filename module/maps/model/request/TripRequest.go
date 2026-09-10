package request

import (
	"strings"

	"Road-To-Destination-BE/module/utils/enum"
)

// TripRequest is Goong Trip v2 — optimize stop order (TSP) then route the result.
// origin, waypoints, and destination are each optional, but together they must
// yield at least 10 coordinates. Default vehicle is car; roundtrip defaults true.
type TripRequest struct {
	Origin      string       `form:"origin"`
	Destination string       `form:"destination"`
	Waypoints   string       `form:"waypoints"`
	Vehicle     enum.Vehicle `form:"vehicle" swaggertype:"string" example:"car"`
	Roundtrip   *bool        `form:"roundtrip"`
	Steps       *bool        `form:"steps"`
}

func (req TripRequest) RoundtripValue() bool {
	if req.Roundtrip == nil {
		return true
	}
	return *req.Roundtrip
}

func (req TripRequest) StepsValue() bool {
	if req.Steps == nil {
		return true
	}
	return *req.Steps
}

func (req TripRequest) PointCount() int {
	n := 0
	if strings.TrimSpace(req.Origin) != "" {
		n++
	}
	if strings.TrimSpace(req.Destination) != "" {
		n++
	}
	for _, point := range strings.Split(req.Waypoints, ";") {
		if strings.TrimSpace(point) != "" {
			n++
		}
	}
	return n
}
