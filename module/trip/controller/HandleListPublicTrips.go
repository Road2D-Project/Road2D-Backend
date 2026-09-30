package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripListResponse{}

// HandleListPublicTrips returns trips with visibility true so someone who is
// not a member can find them. Private trips are omitted. The invite token is
// not included.
//
// @Summary      List public trips
// @Description  Authenticated callers can browse trips marked public. Private trips are hidden. Invite tokens are never returned.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.TripListResponse
// @Failure      401  {object}  share.ErrorResponse
// @Router       /trips/public [get]
func (ctrl *TripController) HandleListPublicTrips() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		list, err := ctrl.tripService().ListPublicTrips(c.Request.Context())
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
