package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleCompleteTrip locks the trip. Leader only.
//
// @Summary      Complete trip
// @Description  Leader confirmation that the trip is finished. An active or stay trip becomes completed and every stored travel is frozen. A planning, locked, or already completed trip is rejected.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip UUID"  format(uuid)
// @Success      200     {object}  response.TripActionResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      409     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/complete [post]
func (ctrl *TripController) HandleCompleteTrip() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		if err := ctrl.tripService().CompleteTrip(c.Request.Context(), tripID); err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.TripActionResponse{Message: "trip completed"})
	}
}
