package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleRespondRequest accepts or rejects a request sent to the caller.
//
// @Summary      Answer a friend request
// @Description  The addressee accepts or rejects. action is accept or reject.
// @Tags         friends
// @Produce      json
// @Security     BearerAuth
// @Param        userId  path      string  true  "Requester UUID"  format(uuid)
// @Param        action  query     string  true  "accept or reject"  Enums(accept, reject)
// @Success      200     {object}  response.FriendRequestResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      409     {object}  share.ErrorResponse
// @Router       /friends/requests/{userId}/respond [post]
func (ctrl *FriendController) HandleRespondRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		otherID, ok := parseUserID(c)
		if !ok {
			return
		}
		accept, ok := parseAcceptOrReject(c)
		if !ok {
			return
		}
		updated, err := ctrl.friendService().Respond(c.Request.Context(), user.ID, otherID, accept)
		if err != nil {
			mapFriendError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
