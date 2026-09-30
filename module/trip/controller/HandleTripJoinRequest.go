package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripMemberResponse{}

// HandleTripJoinRequest approves or rejects a pending add. Accept seats the
// user as a member when the trip is still under its cap.
//
// @Summary      Approve or reject a trip join request
// @Description  Leader or admin. ?action=accept seats the user; reject sets status rejected. Accept fails with 409 when the member limit is already full.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip UUID"  format(uuid)
// @Param        userId  path      string  true  "Applicant UUID"  format(uuid)
// @Param        action  query     string  true  "accept or reject"  Enums(accept, reject)
// @Success      200     {object}  response.TripMemberResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      409     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/join-requests/{userId} [post]
func (ctrl *TripController) HandleTripJoinRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		userID, ok := parseUserID(c)
		if !ok {
			return
		}
		action, ok := parseAcceptOrReject(c)
		if !ok {
			return
		}
		svc := ctrl.tripJoinRequestService()
		var (
			updated *response.TripMemberResponse
			err     error
		)
		if action == membershipActionAccept {
			updated, err = svc.Accept(c.Request.Context(), userID, tripID)
		} else {
			updated, err = svc.Reject(c.Request.Context(), userID, tripID)
		}
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
