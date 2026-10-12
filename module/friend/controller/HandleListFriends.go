package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleListFriends lists accepted friends. Optional q filters usernames with
// the same substring rank as search, only inside this friend list.
//
// @Summary      List friends
// @Description  Accepted friends of the caller, with count. Optional q keeps usernames that contain the query (exact, then prefix, then substring).
// @Tags         friends
// @Produce      json
// @Security     BearerAuth
// @Param        q  query     string  false  "Username filter"
// @Success      200  {object}  response.FriendListResponse
// @Failure      401  {object}  share.ErrorResponse
// @Router       /friends [get]
func (ctrl *FriendController) HandleListFriends() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		list, err := ctrl.friendService().ListFriends(c.Request.Context(), user.ID, c.Query("q"))
		if err != nil {
			mapFriendError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
