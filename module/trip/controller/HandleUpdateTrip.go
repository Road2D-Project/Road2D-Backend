package controller

import (
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripResponse{}

// HandleUpdateTrip applies a leader or admin patch to name, note, times, and visibility.
//
// @Summary      Update trip
// @Description  Leader or admin. Change name, note, startTime, endTime, and/or visibility. Trip type and member limit are not accepted. Status is not updated here.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string                   true  "Trip UUID"  format(uuid)
// @Param        body    body      request.UpdateTripRequest  true  "Fields to update"
// @Success      200     {object}  response.TripResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId} [patch]
func (ctrl *TripController) HandleUpdateTrip() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		role, ok := currentTripRole(c)
		if !ok {
			jsonError(c, http.StatusForbidden, "trip role is missing")
			return
		}
		var req request.UpdateTripRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonBindError(c, err)
			return
		}
		updated, err := ctrl.tripService().UpdateTrip(c.Request.Context(), tripID, req, role)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
