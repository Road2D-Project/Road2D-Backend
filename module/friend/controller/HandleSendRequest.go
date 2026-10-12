package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleSendRequest sends a friend request, or accepts one the other user already sent.
//
// @Summary      Send friend request
// @Description  Creates a pending request. If the other user already sent one, this accepts it. A rejected row is reopened as pending.
// @Tags         friends
// @Produce      json
// @Security     BearerAuth
// @Param        userId  path      string  true  "Other user UUID"  format(uuid)
// @Success      201     {object}  response.FriendRequestResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      409     {object}  share.ErrorResponse
// @Router       /friends/requests/{userId} [post]
func (ctrl *FriendController) HandleSendRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		targetID, ok := parseUserID(c)
		if !ok {
			return
		}
		created, err := ctrl.friendService().SendRequest(c.Request.Context(), user, targetID)
		if err != nil {
			mapFriendError(c, err)
			return
		}
		c.JSON(http.StatusCreated, created)
	}
}
