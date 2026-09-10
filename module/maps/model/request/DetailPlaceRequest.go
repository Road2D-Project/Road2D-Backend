package request

type DetailPlaceRequest struct {
	PlaceId                          string `form:"place_id" binding:"required"`
	HasDeprecatedAdministrativeUnit bool   `form:"has_deprecated_administrative_unit"`
}
