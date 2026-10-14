package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleStartTrip moves a planning trip to active. Leader only.
//
// @Summary      Activate trip
// @Description  Leader only. A planning trip becomes active so realtime tracking can start. A locked, active, stay, or completed trip is rejected.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip UUID"  format(uuid)
// @Success      200     {object}  response.TripActionResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      409     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/active [patch]
func (ctrl *TripController) HandleStartTrip() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		tripId, ok := parseTripID(c)
		if !ok {
			return
		}
		err := ctrl.tripService().StartTrip(c.Request.Context(), tripId)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.TripActionResponse{Message: "trip started"})
	}
}
