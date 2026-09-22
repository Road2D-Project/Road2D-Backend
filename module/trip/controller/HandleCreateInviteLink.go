package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ = response.TripInviteLinkResponse{}

// HandleCreateInviteLink returns the shareable join path. Pass rotate=true to
// mint a new token so previous links stop working.
//
// @Summary      Create trip invite link
// @Description  Leader only. Returns the current invite token and join path. Set rotate=true to invalidate the previous link.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path   string  true   "Trip UUID"  format(uuid)
// @Param        rotate  query  bool    false  "Mint a new token and drop the old one"
// @Success      200     {object}  response.TripInviteLinkResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/invite-link [post]
func (ctrl *TripController) HandleCreateInviteLink() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		rotate := c.Query("rotate") == "true"
		link, err := ctrl.tripService().CreateInviteLink(c.Request.Context(), tripID, rotate)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, link)
	}
}
