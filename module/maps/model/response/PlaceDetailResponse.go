package response

type PlaceDetailResponse struct {
	Result PlaceDetail `json:"result"`
	Status string      `json:"status"`
}

type PlaceDetail struct {
	PlaceID          string          `json:"place_id"`
	Name             string          `json:"name"`
	FormattedAddress string          `json:"formatted_address"`
	Geometry         *PlaceGeometry  `json:"geometry,omitempty"`
}

type PlaceGeometry struct {
	Location LatLng `json:"location"`
}

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}
