package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/trip/model/request"

	"github.com/gin-gonic/gin"
)

// HandleAddDraftDestination parks an existing destination on the trip's draft branch.
// Any active member may call it while the trip is planning. The route graph is unchanged.
//
// @Summary      Add a draft destination
// @Description  Appends one destination to the trip's hidden draft branch. The pin must already exist, usually from POST /planning/fork/{locationId}. Only while the trip is planning. This does not change the route graph or the stored travels.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string                              true  "Trip id"  format(uuid)
// @Param        body    body      request.AddDraftDestinationRequest  true  "Destination to park"
// @Success      200     {object}  response.DraftDestinationsResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/draft [post]
func (ctrl *TripController) HandleAddDraftDestination() gin.HandlerFunc {
	return func(c *gin.Context) {
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		var req request.AddDraftDestinationRequest
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
		listed, err := ctrl.tripBranch().AddDraftDestination(c.Request.Context(), tripID, req.DestinationID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, listed)
	}
}
