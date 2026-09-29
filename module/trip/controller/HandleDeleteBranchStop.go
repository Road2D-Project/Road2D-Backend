package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleDeleteBranchStop removes one destination from a branch and compacts the order.
// The caller must be the leader, and the trip must still be planning. The branch id
// stays. Removing a stop that would disconnect another branch is rejected.
//
// @Summary      Delete a branch stop
// @Description  Removes one destination from a saved branch and rewrites that branch's stops. Split and merge follow the new ends. Only a leader can call this, and only while the trip is planning. A removal that leaves the graph disconnected is rejected and nothing is written.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId         path      string  true  "Trip id"  format(uuid)
// @Param        branchId       path      string  true  "Branch id"  format(uuid)
// @Param        destinationId  path      string  true  "Destination to remove from that branch"  format(uuid)
// @Success      200            {object}  response.TripDetailResponse
// @Failure      400            {object}  share.ErrorResponse
// @Failure      401            {object}  share.ErrorResponse
// @Failure      403            {object}  share.ErrorResponse
// @Failure      404            {object}  share.ErrorResponse
// @Router       /trips/{tripId}/branches/{branchId}/stops/{destinationId} [delete]
func (ctrl *TripController) HandleDeleteBranchStop() gin.HandlerFunc {
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
		detail, err := ctrl.tripBranch().DeleteBranchStop(c.Request.Context(), tripID, branchID, destinationID, role)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}
