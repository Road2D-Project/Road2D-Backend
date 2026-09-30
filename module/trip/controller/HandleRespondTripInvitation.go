package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripMemberResponse{}

// HandleRespondTripInvitation accepts or declines an invitation addressed to
// the caller. Accepting takes a seat only while the trip is under its cap.
//
// @Summary      Accept or decline a trip invitation
// @Description  The invited user responds with ?action=accept|reject. Accept seats them as a member when the trip is not full.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip UUID"  format(uuid)
// @Param        action  query     string  true  "accept or reject"  Enums(accept, reject)
// @Success      200     {object}  response.TripMemberResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      409     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/invitations [post]
func (ctrl *TripController) HandleRespondTripInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		action, ok := parseAcceptOrReject(c)
		if !ok {
			return
		}
		svc := ctrl.tripInvitationService()
		var (
			updated *response.TripMemberResponse
			err     error
		)
		if action == membershipActionAccept {
			updated, err = svc.Accept(c.Request.Context(), user.ID, tripID)
		} else {
			updated, err = svc.Reject(c.Request.Context(), user.ID, tripID)
		}
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
