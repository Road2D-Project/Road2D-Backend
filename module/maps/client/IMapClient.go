package client

import (
	"context"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
)

// IMapClient is the calc REST surface. The only implementation is Goong.
// Do not add a Mapbox REST client — goong-js (Mapbox GL fork) covers map tiles.
// Google Maps Platform is a prohibited territory for Vietnam billing.
type IMapClient interface {
	Autocomplete(ctx context.Context, req request.AutocompleteRequest) (*response.AutocompleteResponse, error)
	DetailPlace(ctx context.Context, req request.DetailPlaceRequest) (*response.PlaceDetailResponse, error)
	Direction(ctx context.Context, req request.DirectionRequest) (*response.DirectionResponse, error)
	Trip(ctx context.Context, req request.TripRequest) (*response.TripResponse, error)
}
