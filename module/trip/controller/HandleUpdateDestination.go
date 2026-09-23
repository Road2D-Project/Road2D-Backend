package controller

import (
	"Road-To-Destination-BE/module/trip/model/request"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = request.UpdateDestinationRequest{}

// HandleUpdateDestination patches a pin that is still editing.
// lat/lng change only when the pin is not linked to a location.
//
// @Summary      Update destination
// @Description  Updates name, arrive time, stay, and status while the destination is editing. lat/lng are applied only when locationId is empty; a forked or verified pin keeps the linked location's coordinates.
// @Tags         planing
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        destinationId  path      string                         true  "Destination UUID"  format(uuid)
// @Param        body           body      request.UpdateDestinationRequest  true  "Fields to store"
// @Success      200            {object}  model.Destination
// @Failure      400            {object}  share.ErrorResponse
// @Failure      401            {object}  share.ErrorResponse
// @Failure      404            {object}  share.ErrorResponse
// @Failure      409            {object}  share.ErrorResponse
// @Router       /planing/destination/{destinationId} [put]
func (ctrl *TripController) HandleUpdateDestination() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		destinationID, ok := parseDestinationID(c)
		if !ok {
			return
		}
		var req request.UpdateDestinationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonBindError(c, err)
			return
		}
		updated, err := ctrl.placeService().UpdateDestination(c.Request.Context(), destinationID, req)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
