package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripResponse{}

// HandleGetTrip loads the trip by id. Active members get myRole. A private trip
// is hidden from everyone else. A public trip is readable without a seat.
//
// @Summary      Get trip
// @Description  Returns trip info without the invite token. Active members get myRole. Private trips are 404 for non-members. Public trips are readable by any authenticated user.
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
