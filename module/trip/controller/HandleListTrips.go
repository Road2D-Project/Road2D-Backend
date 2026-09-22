package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripListResponse{}

// HandleListTrips returns trips where the caller currently holds an active seat.
//
// @Summary      List my trips
// @Description  Returns trips where the caller is an active member (leader or member). Each item includes myRole. Left/kicked seats are omitted.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.TripListResponse
// @Failure      401  {object}  share.ErrorResponse
// @Router       /trips [get]
func (ctrl *TripController) HandleListTrips() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		list, err := ctrl.tripService().ListActiveTripsForUser(c.Request.Context(), user.ID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
