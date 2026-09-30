package controller

import (
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripResponse{}

// HandleForkTrip copies the source trip for the caller, who becomes its leader.
// Active members are copied unless listed in excludeUserIds. The route is not copied.
//
// @Summary      Fork trip
// @Description  Any active member forks the trip. The caller is the new leader. Other active members are copied as members, minus excludeUserIds. Trip type and member limit follow the new trip's tier and cannot be set freely. The route graph is not copied.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string                  true  "Source trip UUID"  format(uuid)
// @Param        body    body      request.ForkTripRequest false "Optional overrides and members to drop"
// @Success      201     {object}  response.TripResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      409     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/fork [post]
func (ctrl *TripController) HandleForkTrip() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		var req request.ForkTripRequest
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			jsonBindError(c, err)
			return
		}
		if ctrl.validator != nil {
			if err := ctrl.validator.Struct(req); err != nil {
				jsonBindError(c, err)
				return
			}
		}
		created, err := ctrl.tripService().ForkTrip(c.Request.Context(), user, tripID, req)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusCreated, created)
	}
}
