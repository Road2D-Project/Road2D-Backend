package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleGetGroup godoc
// @Summary      Get group
// @Description  Returns name and description. If the caller is an active member, myRole is set (owner/admin/member). Non-members still receive public name/description so they can request to join; roster is never included here.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path      string  true  "Group UUID"  format(uuid)
// @Success      200      {object}  response.GroupResponse
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      404      {object}  share.ErrorResponse
// @Router       /groups/{groupId} [get]
func (ctrl *GroupController) HandleGetGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		groupID, ok := parseGroupID(c)
		if !ok {
			return
		}
		got, err := ctrl.groupService().GetGroup(c.Request.Context(), groupID, user.ID)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, got)
	}
}
