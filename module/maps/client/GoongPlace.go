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
	origin := req.Origin
	if origin == "" {
		origin = req.Location
	}
	if origin != "" {
		query.Set("origin", origin)
	}
	if req.Limit > 0 {
		query.Set("limit", strconv.Itoa(req.Limit))
	}
	if req.Radius > 0 {
		query.Set("radius", strconv.Itoa(req.Radius))
	}
	setDeprecatedAdmin(query, req.HasDeprecatedAdministrativeUnit)

	var out response.AutocompleteResponse
	if err := g.getJSON(ctx, "/v2/place/autocomplete", query, &out); err != nil {
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
	setDeprecatedAdmin(query, req.HasDeprecatedAdministrativeUnit)

	var out response.PlaceDetailResponse
	if err := g.getJSON(ctx, "/v2/place/detail", query, &out); err != nil {
		return nil, err
	}
	if goongFailed(out.Status) {
		return nil, fmt.Errorf("%w: %s", ErrGoongStatus, out.Status)
	}
	return &out, nil
}

func setDeprecatedAdmin(query url.Values, include bool) {
	if include {
		query.Set("has_deprecated_administrative_unit", "true")
	}
}
