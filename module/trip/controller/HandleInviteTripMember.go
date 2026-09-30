package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripMemberResponse{}

// HandleInviteTripMember names a user to add. Any account id is accepted until
// a friend graph exists. A leader or admin invite waits for the invitee. A
// member invite waits for a leader or admin to approve it.
//
// @Summary      Invite a user to a trip
// @Description  Any active member may invite by user id. Leader or admin invites are status invited (the invitee accepts). Member invites are status pending until a leader or admin approves. There is no friend check yet.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip UUID"  format(uuid)
// @Param        userId  path      string  true  "User UUID"  format(uuid)
// @Success      200     {object}  response.TripMemberResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      409     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/invite/{userId} [post]
func (ctrl *TripController) HandleInviteTripMember() gin.HandlerFunc {
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
		created, err := ctrl.inviteTripMemberService().Invite(c.Request.Context(), targetID, tripID, user.ID, user.Username, role)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, created)
	}
}
