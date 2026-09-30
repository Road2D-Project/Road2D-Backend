package controller

import (
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripResponse{}

// HandleCreateTrip binds the body, then creates a planning trip. The caller is
// the leader. memberUserIds are seated immediately as members. Trip type freezes
// the member cap. The invite token is not in this response.
//
// @Summary      Create trip
// @Description  Create a planning trip. The caller becomes leader and must name at least one other user, seated as a member. Bronze caps the trip at 15 seats. Trip type and member limit cannot be changed later. Optional mainBranch is the response of POST /planning/preview and becomes the trip's initial main branch. A group chat is not created yet.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      request.CreateTripRequest  true  "Name, trip type, members, optional note, times, visibility, and reviewed main branch"
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
