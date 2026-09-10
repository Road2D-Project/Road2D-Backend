package response

type AutocompleteResponse struct {
	Predictions []PlacePrediction `json:"predictions"`
	Status      string            `json:"status"`
}

type PlacePrediction struct {
	Description           string                   `json:"description"`
	PlaceID               string                   `json:"place_id"`
	StructuredFormatting  *StructuredFormatting   `json:"structured_formatting,omitempty"`
	Compound              *AdministrativeCompound `json:"compound,omitempty"`
	DeprecatedDescription string                   `json:"deprecated_description,omitempty"`
	DeprecatedCompound    *AdministrativeCompound `json:"deprecated_compound,omitempty"`
}

type StructuredFormatting struct {
	MainText      string `json:"main_text"`
	SecondaryText string `json:"secondary_text"`
}

type AdministrativeCompound struct {
	Commune  string `json:"commune,omitempty"`
	District string `json:"district,omitempty"`
	Province string `json:"province,omitempty"`
}
