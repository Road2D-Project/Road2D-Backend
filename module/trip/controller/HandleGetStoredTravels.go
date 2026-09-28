package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/trip/model/response"

	"github.com/gin-gonic/gin"
)

// HandleGetStoredTravels returns the travels already stored on the saved graph.
// Any active member may call it. Nothing is routed or written.
//
// @Summary      Get stored travels
// @Description  Returns the travels already stored for the saved branches, one slice per branch, in stop order. A hop that has not been computed keeps its destination ids and an empty id. Any active member may call this, whether the trip is planning or locked. This does not call the map provider and does not write. POST /trips/{tripId}/travels is the call that computes.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip id"  format(uuid)
// @Success      200     {object}  response.ComputeTripResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/travels [get]
func (ctrl *TripController) HandleGetStoredTravels() gin.HandlerFunc {
	return func(c *gin.Context) {
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		graph, err := ctrl.readStoredTrip().LoadStoredTravels(c.Request.Context(), tripID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.FromTravelGraph(graph))
	}
}
