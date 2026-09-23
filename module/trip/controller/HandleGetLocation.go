package controller

import (
	"Road-To-Destination-BE/module/trip/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = model.Location{}

// HandleGetLocation returns a verified place. Locations are not edited through this API.
//
// @Summary      Get location
// @Description  Returns a verified location. Planning copies it with fork; this row itself is not updated here.
// @Tags         planing
// @Produce      json
// @Security     BearerAuth
// @Param        locationId  path      string  true  "Location UUID"  format(uuid)
// @Success      200         {object}  model.Location
// @Failure      400         {object}  share.ErrorResponse
// @Failure      401         {object}  share.ErrorResponse
// @Failure      404         {object}  share.ErrorResponse
// @Router       /planing/location/{locationId} [get]
func (ctrl *TripController) HandleGetLocation() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		locationID, ok := parseLocationID(c)
		if !ok {
			return
		}
		got, err := ctrl.placeService().GetLocation(c.Request.Context(), locationID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, got)
	}
}
