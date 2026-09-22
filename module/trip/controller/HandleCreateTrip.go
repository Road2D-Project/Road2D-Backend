package controller

import (
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripResponse{}

// HandleCreateTrip binds the body, then creates a planning trip in the given
// group with the caller as leader. The caller must already be an active group member.
//
// @Summary      Create trip
// @Description  Create a planning trip under a group. The caller must be an active group member and becomes the trip leader. An invite token is minted immediately.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      request.CreateTripRequest  true  "Group id, name, optional note and times"
// @Success      201   {object}  response.TripResponse
// @Failure      400   {object}  share.ErrorResponse
// @Failure      401   {object}  share.ErrorResponse
// @Failure      403   {object}  share.ErrorResponse
// @Failure      404   {object}  share.ErrorResponse
// @Router       /trips [post]
func (ctrl *TripController) HandleCreateTrip() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		var req request.CreateTripRequest
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
		created, err := ctrl.tripService().CreateTrip(c.Request.Context(), user, req)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusCreated, created)
	}
}
