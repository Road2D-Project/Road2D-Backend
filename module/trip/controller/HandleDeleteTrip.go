package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleDeleteTrip deletes the trip. Members cascade with the row.
//
// @Summary      Delete trip
// @Description  Leader only. Members cascade-delete with the trip.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path  string  true  "Trip UUID"  format(uuid)
// @Success      204     "Trip deleted"
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId} [delete]
func (ctrl *TripController) HandleDeleteTrip() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		if err := ctrl.tripService().DeleteTrip(c.Request.Context(), tripID); err != nil {
			mapTripError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
