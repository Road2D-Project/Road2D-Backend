package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripInvitationListResponse{}

// HandleListTripInvitations lists trips that invited the caller and still
// wait for accept or decline.
//
// @Summary      List my trip invitations
// @Description  Invitations a leader or admin sent to the caller. Pending join requests are not included.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.TripInvitationListResponse
// @Failure      401  {object}  share.ErrorResponse
// @Router       /trips/invitations [get]
func (ctrl *TripController) HandleListTripInvitations() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		list, err := ctrl.tripInvitationService().List(c.Request.Context(), user.ID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
