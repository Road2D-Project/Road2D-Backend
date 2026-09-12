package request

type AutocompleteRequest struct {
	Input                           string `form:"input" binding:"required"`
	Location                        string `form:"location"`
	Origin                          string `form:"origin"`
	Limit                           int    `form:"limit"`
	Radius                          int    `form:"radius"`
	HasDeprecatedAdministrativeUnit bool   `form:"has_deprecated_administrative_unit"`
}
