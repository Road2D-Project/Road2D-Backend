package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleLeaveTrip marks the caller left. A leaving leader transfers to the
// earliest remaining member; the last leader must delete the trip instead.
//
// @Summary      Leave trip
// @Description  The caller leaves (status left). If the leader leaves, leadership transfers to the earliest-joined remaining member. The last remaining leader must delete the trip instead.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path  string  true  "Trip UUID"  format(uuid)
// @Success      204     "Left the trip"
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      409     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/leave [post]
func (ctrl *TripController) HandleLeaveTrip() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		if err := ctrl.leaveTripService().Leave(c.Request.Context(), user.ID, tripID); err != nil {
			mapTripError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
