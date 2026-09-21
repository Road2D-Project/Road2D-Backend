package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleGetJoinedGroup godoc
// @Summary      List groups I have joined
// @Description  Returns groups where the caller is an active member (owner, admin, or member). Each item includes myRole.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.GroupListResponse
// @Failure      401  {object}  share.ErrorResponse
// @Router       /groups/mine [get]
func (ctrl *GroupController) HandleGetJoinedGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		list, err := ctrl.groupService().ListActiveGroupsForUser(c.Request.Context(), user.ID)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
