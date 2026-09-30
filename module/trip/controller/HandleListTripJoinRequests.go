package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripJoinRequestListResponse{}

// HandleListTripJoinRequests returns people waiting for a leader or admin:
// invite-link requests, and invites sent by a regular member.
//
// @Summary      List trip join requests
// @Description  Leader or admin. Pending rows only.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip UUID"  format(uuid)
// @Success      200     {object}  response.TripJoinRequestListResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/join-requests [get]
func (ctrl *TripController) HandleListTripJoinRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		list, err := ctrl.tripJoinRequestService().List(c.Request.Context(), tripID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
