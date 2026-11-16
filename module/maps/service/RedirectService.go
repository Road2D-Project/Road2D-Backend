package service

import (
	"context"
	"errors"
	"fmt"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/utils/enum"
)

var ErrEmptyRedirectRoute = errors.New("redirect route is empty")

// RedirectService routes one live GPS fix to a stop for the tracking room.
// It calls the map provider directly and never touches the leg cache: the
// origin is a moving point, so a cached row would never be reused.
type RedirectService struct {
	mapClient DirectionMapClient
}

func NewRedirectService(mapClient DirectionMapClient) *RedirectService {
	return &RedirectService{mapClient: mapClient}
}

// Redirect returns the encoded polyline, length in meters, and ride time in
// seconds from (fromLat, fromLng) to (toLat, toLng) by bike.
func (srv *RedirectService) Redirect(ctx context.Context, fromLat, fromLng, toLat, toLng float64) (string, float64, float64, error) {
	req := request.DirectionRequest{
		Origin:       fmt.Sprintf("%f,%f", fromLat, fromLng),
		Destination:  fmt.Sprintf("%f,%f", toLat, toLng),
		Vehicle:      enum.BIKE,
		Alternatives: false,
	}
	raw, err := srv.mapClient.Direction(ctx, req)
	if err != nil {
		return "", 0, 0, err
	}
	if raw == nil || len(raw.Routes) == 0 {
		return "", 0, 0, ErrEmptyRedirectRoute
	}
	leg, err := locationLegFromRoute(req, fromLat, fromLng, toLat, toLng, raw.Routes[0])
	if err != nil {
		return "", 0, 0, err
	}
	if leg.Polyline == "" {
		return "", 0, 0, ErrEmptyRedirectRoute
	}
	return leg.Polyline, leg.DistanceM, leg.DurationS, nil
}
