package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
)

func (g *GoongClient) Trip(ctx context.Context, req request.TripRequest) (*response.TripResponse, error) {
	query := url.Values{}
	if origin := strings.TrimSpace(req.Origin); origin != "" {
		query.Set("origin", origin)
	}
	if destination := strings.TrimSpace(req.Destination); destination != "" {
		query.Set("destination", destination)
	}
	if waypoints := strings.TrimSpace(req.Waypoints); waypoints != "" {
		query.Set("waypoints", waypoints)
	}
	query.Set("vehicle", req.Vehicle.Goong())
	query.Set("roundtrip", strconv.FormatBool(req.RoundtripValue()))
	query.Set("steps", strconv.FormatBool(req.StepsValue()))

	var out response.TripResponse
	if err := g.getJSON(ctx, "/v2/trip", query, &out); err != nil {
		return nil, err
	}
	if goongFailed(out.Code) {
		return nil, fmt.Errorf("%w: %s", ErrGoongStatus, out.Code)
	}
	if len(out.Trips) == 0 {
		return nil, ErrEmptyTrip
	}
	return &out, nil
}
