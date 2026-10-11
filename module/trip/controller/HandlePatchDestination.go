package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/trip/model/request"

	"github.com/gin-gonic/gin"
)

var _ = request.PatchDestinationRequest{}

// HandlePatchDestination updates the fields sent for a pin that is still editing.
// lat/lng change only when the pin is not linked to a location.
//
// @Summary      Patch destination
// @Description  Updates any of name, arrive time, stay minutes, and status while the destination is editing. lat/lng are applied only when locationId is empty. Omitted fields stay as they are.
// @Tags         planning
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        destinationId  path      string                          true  "Destination UUID"  format(uuid)
// @Param        body           body      request.PatchDestinationRequest  true  "Fields to update"
// @Success      200            {object}  model.Destination
// @Failure      400            {object}  share.ErrorResponse
// @Failure      401            {object}  share.ErrorResponse
// @Failure      404            {object}  share.ErrorResponse
// @Failure      409            {object}  share.ErrorResponse
// @Router       /planning/destination/{destinationId} [patch]
func (ctrl *TripController) HandlePatchDestination() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		destinationID, ok := parseDestinationID(c)
		if !ok {
			return
		}
		var req request.PatchDestinationRequest
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
		updated, err := ctrl.placeService().PatchDestination(c.Request.Context(), destinationID, req)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
