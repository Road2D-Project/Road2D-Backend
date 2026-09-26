package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/trip/model/request"

	"github.com/gin-gonic/gin"
)

// HandleComputeTrip previews the route along one branch of destinations and locations.
// Any active member may call it. Nothing is written to the trip or to Postgres;
// a cache miss is kept in Redis for 15 minutes.
//
// @Summary      Preview a branch route
// @Description  Compute the bike route along one ordered list of stops. Each point is either a destinationId or a locationId. The result is not stored on the trip. A cache miss is kept in Redis for 15 minutes. Any active member may call this, and the trip does not have to be planning.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string                        true  "Trip id"  format(uuid)
// @Param        body    body      request.ComputeBranchRequest  true  "Ordered stops, each a destination or a location"
// @Success      200     {object}  response.ComputeBranchResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      429     {object}  share.ErrorResponse
// @Failure      500     {object}  share.ErrorResponse
// @Failure      502     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/compute [post]
func (ctrl *TripController) HandleComputeTrip() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := parseTripID(c); !ok {
			return
		}
		var req request.ComputeBranchRequest
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
		out, err := ctrl.computeTrip().PreviewBranch(c.Request.Context(), req.Points)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
	}
}
