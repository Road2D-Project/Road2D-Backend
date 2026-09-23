package controller

import (
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.CreateDestinationResponse{}

// HandleForkLocation copies a verified location into a new editing destination.
// The new pin keeps that location's coordinates, so later updates cannot move it.
//
// @Summary      Fork location
// @Description  Creates an editing destination from a verified location. lat/lng and the location link are copied from the location. Optional name overrides the location name.
// @Tags         planing
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        locationId  path      string                     true  "Location UUID"  format(uuid)
// @Param        body        body      request.ForkLocationRequest  true  "Optional name, arrive time, and stay"
// @Success      201         {object}  response.CreateDestinationResponse
// @Failure      400         {object}  share.ErrorResponse
// @Failure      401         {object}  share.ErrorResponse
// @Failure      404         {object}  share.ErrorResponse
// @Router       /planing/fork/{locationId} [post]
func (ctrl *TripController) HandleForkLocation() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		locationID, ok := parseLocationID(c)
		if !ok {
			return
		}
		var req request.ForkLocationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonBindError(c, err)
			return
		}
		created, err := ctrl.placeService().ForkLocation(c.Request.Context(), locationID, req)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusCreated, created)
	}
}
