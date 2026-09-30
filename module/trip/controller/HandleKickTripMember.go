package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleKickTripMember removes an active member. The leader cannot be kicked.
// An admin cannot kick another admin.
//
// @Summary      Kick trip member
// @Description  Leader or admin. Cannot kick yourself, the leader, or (as an admin) another admin. The row stays as kicked so the user can be invited again.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path  string  true  "Trip UUID"  format(uuid)
// @Param        userId  path  string  true  "Member UUID"  format(uuid)
// @Success      204     "Member kicked"
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/members/{userId} [delete]
func (ctrl *TripController) HandleKickTripMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		targetID, ok := parseUserID(c)
		if !ok {
			return
		}
		role, ok := currentTripRole(c)
		if !ok {
			jsonError(c, http.StatusForbidden, "trip role is missing")
			return
		}
		if err := ctrl.kickTripMemberService().Kick(c.Request.Context(), user.ID, targetID, tripID, role); err != nil {
			mapTripError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
