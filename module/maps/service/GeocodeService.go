package service

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
)

var ErrGeocodeLookup = errors.New("provide exactly one of address, latlng, place_id")

type GeocodeMapClient interface {
	Geocode(ctx context.Context, req request.GeocodeRequest) (*response.GeocodeResponse, error)
}

type GeocodeService struct {
	mapClient GeocodeMapClient
}

func NewGeocodeService(mapClient GeocodeMapClient) *GeocodeService {
	return &GeocodeService{mapClient: mapClient}
}

func (srv *GeocodeService) Lookup(ctx context.Context, req request.GeocodeRequest) (*response.GeocodeResponse, error) {
	if req.LookupCount() != 1 {
		return nil, ErrGeocodeLookup
	}
	return srv.mapClient.Geocode(ctx, req)
}
