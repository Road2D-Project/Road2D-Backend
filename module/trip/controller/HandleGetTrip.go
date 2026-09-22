package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripResponse{}

// HandleGetTrip loads the trip by id. Active members get myRole; others still
// receive the public name/status so they can decide to join via an invite link.
//
// @Summary      Get trip
// @Description  Returns trip info. If the caller is an active member, myRole is set (leader/member). The invite token is never included.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip UUID"  format(uuid)
// @Success      200     {object}  response.TripResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId} [get]
func (ctrl *TripController) HandleGetTrip() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		got, err := ctrl.tripService().GetTrip(c.Request.Context(), tripID, user.ID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, got)
	}
}
