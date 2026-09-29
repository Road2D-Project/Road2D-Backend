package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/trip/model/request"

	"github.com/gin-gonic/gin"
)

// HandleUpdateBranchStop replaces the destination at one stop, moves it, or both.
// The caller must be the leader, and the trip must still be planning. The branch id
// stays. A change that disconnects the graph is rejected.
//
// @Summary      Update a branch stop
// @Description  Changes one stop on a saved branch. destinationId replaces the pin. orderInBranch moves it within that branch, where 0 is the first stop. At least one field is required. Split and merge are derived again from the branch ends. Only a leader can call this, and only while the trip is planning.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tripId         path      string                          true  "Trip id"  format(uuid)
// @Param        branchId       path      string                          true  "Branch id"  format(uuid)
// @Param        destinationId  path      string                          true  "Destination currently at that stop"  format(uuid)
// @Param        body           body      request.UpdateBranchStopRequest true  "Replacement pin and/or new order"
// @Success      200            {object}  response.TripDetailResponse
// @Failure      400            {object}  share.ErrorResponse
// @Failure      401            {object}  share.ErrorResponse
// @Failure      403            {object}  share.ErrorResponse
// @Failure      404            {object}  share.ErrorResponse
// @Router       /trips/{tripId}/branches/{branchId}/stops/{destinationId} [patch]
func (ctrl *TripController) HandleUpdateBranchStop() gin.HandlerFunc {
	return func(c *gin.Context) {
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		branchID, ok := parseBranchID(c)
		if !ok {
			return
		}
		destinationID, ok := parseDestinationID(c)
		if !ok {
			return
		}
		role, ok := currentTripRole(c)
		if !ok {
			jsonError(c, http.StatusForbidden, "trip role is missing")
			return
		}
		var req request.UpdateBranchStopRequest
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
		detail, err := ctrl.tripBranch().UpdateBranchStop(c.Request.Context(), tripID, branchID, destinationID, req, role)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}
