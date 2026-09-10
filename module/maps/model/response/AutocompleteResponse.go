package response

type AutocompleteResponse struct {
	Predictions []PlacePrediction `json:"predictions"`
	Status      string            `json:"status"`
}

type PlacePrediction struct {
	Description         string               `json:"description"`
	PlaceID             string               `json:"place_id"`
	StructuredFormatting *StructuredFormatting `json:"structured_formatting,omitempty"`
}

type StructuredFormatting struct {
	MainText      string `json:"main_text"`
	SecondaryText string `json:"secondary_text"`
}
