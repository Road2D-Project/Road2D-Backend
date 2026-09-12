package response

type GeocodeResponse struct {
	Results []GeocodeResult `json:"results"`
	Status  string          `json:"status"`
}

type GeocodeResult struct {
	AddressComponents     []AddressComponent         `json:"address_components"`
	FormattedAddress      string                      `json:"formatted_address"`
	Geometry              *GeocodeGeometry           `json:"geometry,omitempty"`
	PlaceID               string                      `json:"place_id"`
	Reference             string                      `json:"reference,omitempty"`
	PlusCode              *PlusCode                 `json:"plus_code,omitempty"`
	Compound              *AdministrativeCompound      `json:"compound,omitempty"`
	Types                 []string                   `json:"types"`
	Name                  string                      `json:"name,omitempty"`
	Address               string                      `json:"address,omitempty"`
	DeprecatedDescription string                      `json:"deprecated_description,omitempty"`
	DeprecatedCompound    *AdministrativeCompound      `json:"deprecated_compound,omitempty"`
	DeprecatedCompoundID *AdministrativeCompoundID     `json:"deprecated_compound_id,omitempty"`
}

type GeocodeGeometry struct {
	Location LatLng  `json:"location"`
	Boundary *string `json:"boundary"`
}

type AddressComponent struct {
	LongName  string `json:"long_name"`
	ShortName string `json:"short_name"`
}

type PlusCode struct {
	CompoundCode string `json:"compound_code,omitempty"`
	GlobalCode   string `json:"global_code,omitempty"`
}

type AdministrativeCompoundID struct {
	Commune  int `json:"commune,omitempty"`
	District int `json:"district,omitempty"`
	Province int `json:"province,omitempty"`
}
