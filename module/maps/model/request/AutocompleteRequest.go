package request

type AutocompleteRequest struct {
	Input                            string `form:"input" binding:"required"`
	Location                         string `form:"location"`
	Limit                            int    `form:"limit"`
	HasDeprecatedAdministrativeUnit bool   `form:"has_deprecated_administrative_unit"`
}
