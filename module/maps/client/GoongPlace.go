package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
)

func (g *GoongClient) Autocomplete(ctx context.Context, req request.AutocompleteRequest) (*response.AutocompleteResponse, error) {
	query := url.Values{}
	query.Set("input", req.Input)
	if req.Location != "" {
		query.Set("location", req.Location)
	}
	if req.SessionToken != "" {
		query.Set("sessiontoken", req.SessionToken)
	}
	if req.Limit > 0 {
		query.Set("limit", strconv.Itoa(req.Limit))
	}

	var out response.AutocompleteResponse
	if err := g.getJSON(ctx, "/Place/AutoComplete", query, &out); err != nil {
		return nil, err
	}
	if goongFailed(out.Status) {
		return nil, fmt.Errorf("%w: %s", ErrGoongStatus, out.Status)
	}
	return &out, nil
}

func (g *GoongClient) DetailPlace(ctx context.Context, req request.DetailPlaceRequest) (*response.PlaceDetailResponse, error) {
	query := url.Values{}
	query.Set("place_id", req.PlaceId)
	if req.SessionToken != "" {
		query.Set("sessiontoken", req.SessionToken)
	}

	var out response.PlaceDetailResponse
	if err := g.getJSON(ctx, "/Place/Detail", query, &out); err != nil {
		return nil, err
	}
	if goongFailed(out.Status) {
		return nil, fmt.Errorf("%w: %s", ErrGoongStatus, out.Status)
	}
	return &out, nil
}
