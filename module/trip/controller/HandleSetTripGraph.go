package controller

import (
	"Road-To-Destination-BE/module/trip/model/request"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleSetTripGraph replaces the whole route graph of a planning trip. The
// caller must be the trip leader. The body lists destination ids per branch;
// the response is the trip with its branches reduced to render fields.
//
// @Summary      Set trip graph
// @Description  Replace the route graph of a planning trip. branches[0] is the main branch. Every later branch splits at its first destination and merges at its last, unless openTail marks that branch as ending without rejoining. Only a leader can edit, and only while the trip is planning.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string                      true  "Trip id"  format(uuid)
// @Param        body    body      request.SetTripGraphRequest true  "Branches of destination ids and which tails stay open"
// @Success      200     {object}  response.TripDetailResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/graph [put]
func (ctrl *TripController) HandleSetTripGraph() gin.HandlerFunc {
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
		var req request.SetTripGraphRequest
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
		detail, err := ctrl.tripBranch().SetTripGraph(c.Request.Context(), tripID, req, role)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}
