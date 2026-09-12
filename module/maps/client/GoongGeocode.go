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

func (g *GoongClient) Geocode(ctx context.Context, req request.GeocodeRequest) (*response.GeocodeResponse, error) {
	query := url.Values{}
	if address := req.AddressValue(); address != "" {
		query.Set("address", address)
	}
	if latlng := req.LatLngValue(); latlng != "" {
		query.Set("latlng", latlng)
	}
	if placeID := req.PlaceIDValue(); placeID != "" {
		query.Set("place_id", placeID)
	}
	if req.Limit > 0 {
		query.Set("limit", strconv.Itoa(req.Limit))
	}
	setDeprecatedAdmin(query, req.HasDeprecatedAdministrativeUnit)
	if req.HasVNID {
		query.Set("has_vnid", "true")
	}

	var out response.GeocodeResponse
	if err := g.getJSON(ctx, "/v2/geocode", query, &out); err != nil {
		return nil, err
	}
	if geocodeFailed(out.Status) {
		return nil, fmt.Errorf("%w: %s", ErrGoongStatus, out.Status)
	}
	return &out, nil
}

func geocodeFailed(status string) bool {
	if status == "" || strings.EqualFold(status, "OK") || strings.EqualFold(status, "ZERO_RESULTS") {
		return false
	}
	return true
}
