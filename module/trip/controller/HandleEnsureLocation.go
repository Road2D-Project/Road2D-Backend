package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"

	"github.com/gin-gonic/gin"
)

var _ = response.EnsureLocationResponse{}

// HandleEnsureLocation stores a Goong place in the catalog, or returns the row already stored.
// The locationId in the response is what POST /planning/preview accepts.
//
// @Summary      Ensure location
// @Description  Resolve a Goong place_id into a catalog location. An existing place_id is returned as-is. A new one is loaded from Goong Place Detail and stored. Pass locationId to POST /planning/preview. This does not create a destination.
// @Tags         planning
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      request.EnsureLocationRequest  true  "Goong place id from autocomplete"
// @Success      200   {object}  response.EnsureLocationResponse  "Place was already in the catalog"
// @Success      201   {object}  response.EnsureLocationResponse  "Place was stored"
// @Failure      400   {object}  share.ErrorResponse
// @Failure      401   {object}  share.ErrorResponse
// @Failure      429   {object}  share.ErrorResponse
// @Failure      500   {object}  share.ErrorResponse
// @Failure      502   {object}  share.ErrorResponse
// @Router       /planning/location [post]
func (ctrl *TripController) HandleEnsureLocation() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		var req request.EnsureLocationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonBindError(c, err)
			return
		}
		if ctrl.validator != nil {
			if err := ctrl.validator.Struct(req); err != nil {
				jsonBindError(c, err)
				return
			}
		}
		out, created, err := ctrl.placeService().EnsureLocation(c.Request.Context(), req.PlaceID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		c.JSON(status, out)
	}
}
