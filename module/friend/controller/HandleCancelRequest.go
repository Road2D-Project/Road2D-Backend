package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleCancelRequest deletes a pending request the caller sent.
//
// @Summary      Cancel friend request
// @Description  Deletes a pending request sent by the caller.
// @Tags         friends
// @Produce      json
// @Security     BearerAuth
// @Param        userId  path  string  true  "Addressee UUID"  format(uuid)
// @Success      204
// @Failure      400  {object}  share.ErrorResponse
// @Failure      401  {object}  share.ErrorResponse
// @Failure      404  {object}  share.ErrorResponse
// @Router       /friends/requests/{userId} [delete]
func (ctrl *FriendController) HandleCancelRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		otherID, ok := parseUserID(c)
		if !ok {
			return
		}
		if err := ctrl.friendService().CancelRequest(c.Request.Context(), user.ID, otherID); err != nil {
			mapFriendError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
