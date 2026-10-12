package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleListRequests lists pending friend requests.
//
// @Summary      List friend requests
// @Description  Pending requests. box=in (default) is waiting on the caller. box=out is requests the caller sent.
// @Tags         friends
// @Produce      json
// @Security     BearerAuth
// @Param        box  query     string  false  "in or out"  Enums(in, out)
// @Success      200  {object}  response.FriendRequestListResponse
// @Failure      400  {object}  share.ErrorResponse
// @Failure      401  {object}  share.ErrorResponse
// @Router       /friends/requests [get]
func (ctrl *FriendController) HandleListRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		incoming, ok := parseRequestBox(c)
		if !ok {
			return
		}
		list, err := ctrl.friendService().ListRequests(c.Request.Context(), user.ID, incoming)
		if err != nil {
			mapFriendError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
