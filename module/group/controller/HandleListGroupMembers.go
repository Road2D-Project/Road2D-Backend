package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleListGroupMembers godoc
// @Summary      List group members
// @Description  Returns the active roster (owner, admin, member). Invited, pending, left, rejected, and kicked rows are omitted. Any active member may call this.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path      string  true  "Group UUID"  format(uuid)
// @Success      200      {object}  response.GroupMemberListResponse
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      403      {object}  share.ErrorResponse
// @Router       /groups/{groupId}/members [get]
func (ctrl *GroupController) HandleListGroupMembers() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		groupId, ok := parseGroupID(c)
		if !ok {
			return
		}
		list, err := ctrl.groupMemberService().ListActiveMembers(c.Request.Context(), groupId)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
