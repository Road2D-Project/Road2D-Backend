package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripMemberListResponse{}

// HandleListTripMembers returns the active roster, leader first.
//
// @Summary      List trip members
// @Description  Active members only. Pending invites and join requests are separate lists.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip UUID"  format(uuid)
// @Success      200     {object}  response.TripMemberListResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/members [get]
func (ctrl *TripController) HandleListTripMembers() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		list, err := ctrl.tripMemberService().ListActiveMembers(c.Request.Context(), tripID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
