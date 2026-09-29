package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"

	"github.com/gin-gonic/gin"
)

// HandleComputeTravelGraph routes a graph of destination ids and stores the travels.
// The body is the route. Saved branch rows are not read back to decide the hops.
// The caller must be the leader, and the trip must still be planning.
//
// @Summary      Compute travels from destination ids
// @Description  Route a graph sent as destination ids, one inner array per branch in stop order, and upsert the travels. This does not replace the stored branches. Only a leader can call this, and only while the trip is planning. POST /trips/{tripId}/travels still routes whatever branches are already saved.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string                             true  "Trip id"  format(uuid)
// @Param        body    body      request.ComputeTravelGraphRequest  true  "Branches of destination ids"
// @Success      200     {object}  response.ComputeTripResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      429     {object}  share.ErrorResponse
// @Failure      500     {object}  share.ErrorResponse
// @Failure      502     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/travels/graph [post]
func (ctrl *TripController) HandleComputeTravelGraph() gin.HandlerFunc {
	return func(c *gin.Context) {
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		var req request.ComputeTravelGraphRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonBindError(c, err)
			return
		}
		if ctrl.validator != nil {
			if err := ctrl.validator.Struct(req); err != nil {
				jsonBindError(c, err)
				return
			}
		}
		graph, err := ctrl.computeStoredTrip().ComputeTravelGraph(c.Request.Context(), tripID, req.Branches)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.FromTravelGraph(graph))
	}
}
