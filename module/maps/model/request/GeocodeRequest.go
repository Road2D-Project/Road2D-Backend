package request

import "strings"

// GeocodeRequest is Goong Geocode v2. Provide exactly one of address, latlng, or place_id.
type GeocodeRequest struct {
	Address                         string `form:"address"`
	LatLng                          string `form:"latlng"`
	PlaceID                         string `form:"place_id"`
	Limit                           int    `form:"limit"`
	HasDeprecatedAdministrativeUnit bool `form:"has_deprecated_administrative_unit"`
	HasVNID                         bool   `form:"has_vnid"`
}

func (req GeocodeRequest) AddressValue() string {
	return strings.TrimSpace(req.Address)
}

func (req GeocodeRequest) PlaceIDValue() string {
	return strings.TrimSpace(req.PlaceID)
}

func (req GeocodeRequest) LatLngValue() string {
	raw := strings.TrimSpace(req.LatLng)
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 2 {
		return raw
	}
	return strings.TrimSpace(parts[0]) + "," + strings.TrimSpace(parts[1])
}

func (req GeocodeRequest) LookupCount() int {
	n := 0
	if req.AddressValue() != "" {
		n++
	}
	if req.LatLngValue() != "" {
		n++
	}
	if req.PlaceIDValue() != "" {
		n++
	}
	return n
}
