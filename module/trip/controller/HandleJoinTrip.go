package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var _ = response.TripMemberResponse{}

// HandleJoinTrip looks up the trip by invite token and records a pending join
// request. A leader or admin must approve it before the caller has a seat.
//
// @Summary      Request to join a trip via invite link
// @Description  Authenticated caller opens a pending join request for the invite token. A leader or admin approves it. Already-active members, existing invites, and a full trip return 409.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        token  path      string  true  "Invite token (UUID)"
// @Success      200    {object}  response.TripMemberResponse
// @Failure      400    {object}  share.ErrorResponse
// @Failure      401    {object}  share.ErrorResponse
// @Failure      404    {object}  share.ErrorResponse
// @Failure      409    {object}  share.ErrorResponse
// @Router       /trips/join/{token} [post]
func (ctrl *TripController) HandleJoinTrip() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		token := strings.TrimSpace(c.Param("token"))
		if _, err := uuid.Parse(token); err != nil {
			jsonError(c, http.StatusBadRequest, "invalid invite token")
			return
		}
		joined, err := ctrl.joinTripService().Join(c.Request.Context(), user, token)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, joined)
	}
}
