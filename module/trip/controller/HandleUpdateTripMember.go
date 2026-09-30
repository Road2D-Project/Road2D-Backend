package controller

import (
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripMemberResponse{}

// HandleUpdateTripMember changes the caller's nickname, or lets the leader set
// another member's role to admin or member.
//
// @Summary      Update trip member
// @Description  An active member may change only their own nickname. The leader may also set another member's role to admin or member. Leadership is not assigned here.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string                           true  "Trip UUID"  format(uuid)
// @Param        userId  path      string                           true  "Member UUID"  format(uuid)
// @Param        body    body      request.UpdateTripMemberRequest  true  "Nickname and/or role"
// @Success      200     {object}  response.TripMemberResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/members/{userId} [patch]
func (ctrl *TripController) HandleUpdateTripMember() gin.HandlerFunc {
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
		var req request.UpdateTripMemberRequest
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
		updated, err := ctrl.tripMemberService().Update(c.Request.Context(), user.ID, targetID, tripID, role, req)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
