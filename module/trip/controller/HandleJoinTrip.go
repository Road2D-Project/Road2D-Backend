package controller

import (
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var _ = response.TripResponse{}

// HandleJoinTrip looks up the trip by invite token and seats the caller as an
// active member. A left row is reused; an already-active seat is a conflict.
//
// @Summary      Join trip via invite link
// @Description  Authenticated caller joins the trip identified by the invite token as a member. Reuses a left/rejected/kicked row. Already-active members get 409.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        token  path      string  true  "Invite token (UUID)"
// @Success      200    {object}  response.TripResponse
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
