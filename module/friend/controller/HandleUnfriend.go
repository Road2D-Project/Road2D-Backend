package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleUnfriend removes an accepted friendship.
//
// @Summary      Unfriend
// @Description  Deletes an accepted friendship. The friend count is the number of remaining accepted rows.
// @Tags         friends
// @Produce      json
// @Security     BearerAuth
// @Param        userId  path  string  true  "Friend UUID"  format(uuid)
// @Success      204
// @Failure      400  {object}  share.ErrorResponse
// @Failure      401  {object}  share.ErrorResponse
// @Failure      404  {object}  share.ErrorResponse
// @Router       /friends/{userId} [delete]
func (ctrl *FriendController) HandleUnfriend() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		otherID, ok := parseUserID(c)
		if !ok {
			return
		}
		if err := ctrl.friendService().Unfriend(c.Request.Context(), user.ID, otherID); err != nil {
			mapFriendError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
