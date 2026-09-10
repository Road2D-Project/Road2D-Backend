package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
)

func (g *GoongClient) Direction(ctx context.Context, req request.DirectionRequest) (*response.DirectionResponse, error) {
	query := url.Values{}
	query.Set("origin", req.Origin)
	query.Set("destination", req.Destination)
	query.Set("vehicle", req.Vehicle.Goong())
	if req.Alternatives {
		query.Set("alternatives", strconv.FormatBool(true))
	}

	var out response.DirectionResponse
	if err := g.getJSON(ctx, "/v2/direction", query, &out); err != nil {
		return nil, err
	}
	if goongFailed(out.Status) {
		return nil, fmt.Errorf("%w: %s", ErrGoongStatus, out.Status)
	}
	if len(out.Routes) == 0 {
		return nil, ErrEmptyRoute
	}
	return &out, nil
}
