package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleSearchUsers finds accounts to befriend.
// Username matching is a substring rank (exact, prefix, contains), not typo-tolerant.
// A query that is only a phone number matches users.number_phone by full equality.
// Partial numbers do not match. The phone itself is not returned.
//
// Upgrade later: pg_trgm similarity and a GIN index, display name, cursor
// pagination, rate limiting, and a block list. Do not fuzzy-match email or phone.
//
// @Summary      Search users
// @Description  Ranked username substring search, at most 20 rows. A full phone number matches number_phone exactly and is not included in the response. Each row has relation none, outgoing, incoming, or friends.
// @Tags         friends
// @Produce      json
// @Security     BearerAuth
// @Param        q  query     string  true  "Username fragment, or a full phone number"
// @Success      200  {object}  response.FriendSearchResponse
// @Failure      400  {object}  share.ErrorResponse
// @Failure      401  {object}  share.ErrorResponse
// @Router       /friends/search [get]
func (ctrl *FriendController) HandleSearchUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		list, err := ctrl.friendService().Search(c.Request.Context(), user.ID, c.Query("q"))
		if err != nil {
			mapFriendError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
