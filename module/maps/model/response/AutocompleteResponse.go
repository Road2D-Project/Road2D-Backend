package response

type AutocompleteResponse struct {
	Predictions   []PlacePrediction `json:"predictions"`
	ExecutionTime string            `json:"execution_time,omitempty"`
	Status        string            `json:"status"`
}

type PlacePrediction struct {
	Description           string                  `json:"description"`
	PlaceID               string                  `json:"place_id"`
	Reference             string                  `json:"reference,omitempty"`
	MatchedSubstrings     []MatchedSubstring      `json:"matched_substrings,omitempty"`
	StructuredFormatting  *StructuredFormatting   `json:"structured_formatting,omitempty"`
	HasChildren           bool                    `json:"has_children,omitempty"`
	PlusCode              *PlusCode               `json:"plus_code,omitempty"`
	Compound              *AdministrativeCompound `json:"compound,omitempty"`
	Terms                 []PlaceTerm             `json:"terms,omitempty"`
	Types                 []string                `json:"types,omitempty"`
	DistanceMeters        *int                    `json:"distance_meters,omitempty"`
	DeprecatedDescription string                  `json:"deprecated_description,omitempty"`
	DeprecatedCompound    *AdministrativeCompound `json:"deprecated_compound,omitempty"`
}

type MatchedSubstring struct {
	Length int `json:"length"`
	Offset int `json:"offset"`
}

type PlaceTerm struct {
	Offset int    `json:"offset"`
	Value  string `json:"value"`
}

type StructuredFormatting struct {
	MainText                       string             `json:"main_text"`
	MainTextMatchedSubstrings      []MatchedSubstring `json:"main_text_matched_substrings,omitempty"`
	SecondaryText                  string             `json:"secondary_text"`
	SecondaryTextMatchedSubstrings []MatchedSubstring `json:"secondary_text_matched_substrings,omitempty"`
}

type AdministrativeCompound struct {
	Commune  string `json:"commune,omitempty"`
	District string `json:"district,omitempty"`
	Province string `json:"province,omitempty"`
}
