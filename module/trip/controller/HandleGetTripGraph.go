package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/trip/model/response"

	"github.com/gin-gonic/gin"
)

// HandleGetTripGraph returns the trip, its branches, and the travels stored on each branch.
// Any active member may call it, including after the trip is locked. It does not recompute.
//
// @Summary      Get trip graph
// @Description  Returns the trip, its saved branches, and the travels already stored on each branch. travels[i] is the hop from stops[i] to stops[i+1]; null means that hop has not been computed. Any active member may call this, whether the trip is planning or locked. This does not recompute routes.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string  true  "Trip id"  format(uuid)
// @Success      200     {object}  response.TripDetailResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/graph [get]
func (ctrl *TripController) HandleGetTripGraph() gin.HandlerFunc {
	return func(c *gin.Context) {
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		role, ok := currentTripRole(c)
		if !ok {
			jsonError(c, http.StatusForbidden, "trip role is missing")
			return
		}
		detail, err := ctrl.tripBranch().GetTripGraph(c.Request.Context(), tripID, role)
		if err != nil {
			mapTripError(c, err)
			return
		}
		stored, err := ctrl.readStoredTrip().LoadStoredTravels(c.Request.Context(), tripID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.AttachTravels(*detail, stored))
	}
}
