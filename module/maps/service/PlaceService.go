package service

import (
	"context"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
)

type PlaceMapClient interface {
	Autocomplete(ctx context.Context, req request.AutocompleteRequest) (*response.AutocompleteResponse, error)
	DetailPlace(ctx context.Context, req request.DetailPlaceRequest) (*response.PlaceDetailResponse, error)
}

type PlaceService struct {
	mapClient PlaceMapClient
}

func NewPlaceService(mapClient PlaceMapClient) *PlaceService {
	return &PlaceService{mapClient: mapClient}
}

func (srv *PlaceService) Autocomplete(ctx context.Context, req request.AutocompleteRequest) (*response.AutocompleteResponse, error) {
	return srv.mapClient.Autocomplete(ctx, req)
}

func (srv *PlaceService) GetDetailPlace(ctx context.Context, req request.DetailPlaceRequest) (*response.PlaceDetailResponse, error) {
	return srv.mapClient.DetailPlace(ctx, req)
}
