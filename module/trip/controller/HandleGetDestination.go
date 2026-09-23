package controller

import (
	"Road-To-Destination-BE/module/trip/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = model.Destination{}

// HandleGetDestination returns a plan pin, including the location it is linked to.
//
// @Summary      Get destination
// @Description  Returns a destination. When locationId is set, lat/lng are locked to that location.
// @Tags         planing
// @Produce      json
// @Security     BearerAuth
// @Param        destinationId  path      string  true  "Destination UUID"  format(uuid)
// @Success      200            {object}  model.Destination
// @Failure      400            {object}  share.ErrorResponse
// @Failure      401            {object}  share.ErrorResponse
// @Failure      404            {object}  share.ErrorResponse
// @Router       /planing/destination/{destinationId} [get]
func (ctrl *TripController) HandleGetDestination() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		destinationID, ok := parseDestinationID(c)
		if !ok {
			return
		}
		got, err := ctrl.placeService().GetDestination(c.Request.Context(), destinationID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, got)
	}
}
