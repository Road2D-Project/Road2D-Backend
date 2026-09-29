package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleListDraftDestinations returns pins parked on the trip's draft branch.
// Any active member may call it. The route graph and stored travels leave this branch out.
//
// @Summary      List draft destinations
// @Description  Returns every destination stored on the trip's hidden draft branch, in inbox order. This branch is not routed and is omitted from GET /trips/{tripId}/graph and GET /trips/{tripId}/travels. Any active member may call this.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip id"  format(uuid)
// @Success      200     {object}  response.DraftDestinationsResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/draft [get]
func (ctrl *TripController) HandleListDraftDestinations() gin.HandlerFunc {
	return func(c *gin.Context) {
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		listed, err := ctrl.tripBranch().ListDraftDestinations(c.Request.Context(), tripID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, listed)
	}
}
