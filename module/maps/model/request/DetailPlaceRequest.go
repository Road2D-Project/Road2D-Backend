package request

type DetailPlaceRequest struct {
	PlaceId      string `form:"place_id" binding:"required"`
	SessionToken string `form:"sessiontoken"`
}
