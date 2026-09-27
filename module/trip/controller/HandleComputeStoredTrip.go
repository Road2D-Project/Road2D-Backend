package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/trip/model/response"

	"github.com/gin-gonic/gin"
)

// HandleComputeStoredTrip routes the trip's saved graph and stores the travels.
// The caller must be the leader, and the trip must still be planning.
// Fresh legs go through Redis and then Postgres. A frozen travel is left as it is.
//
// @Summary      Compute the saved trip graph
// @Description  Route every consecutive pair on the trip's stored branches and upsert the travels. Only a leader can call this, and only while the trip is planning. Reused frozen rows are not rewritten. This is not the preview at POST /trips/{tripId}/compute.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip id"  format(uuid)
// @Success      200     {object}  response.ComputeTripResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      429     {object}  share.ErrorResponse
// @Failure      500     {object}  share.ErrorResponse
// @Failure      502     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/travels [post]
func (ctrl *TripController) HandleComputeStoredTrip() gin.HandlerFunc {
	return func(c *gin.Context) {
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		graph, err := ctrl.computeStoredTrip().ComputeStoredTrip(c.Request.Context(), tripID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.FromTravelGraph(graph))
	}
}
