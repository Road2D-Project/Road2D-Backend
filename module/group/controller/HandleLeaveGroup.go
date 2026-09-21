package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleLeaveGroup godoc
// @Summary      Leave group
// @Description  The caller leaves (status left). If the owner leaves, ownership transfers to the earliest-joined admin, or the earliest-joined member if there is no admin. The last remaining owner must delete the group instead.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path  string  true  "Group UUID"  format(uuid)
// @Success      204      "Left the group"
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      403      {object}  share.ErrorResponse
// @Failure      409      {object}  share.ErrorResponse
// @Router       /groups/{groupId}/leave [post]
func (ctrl *GroupController) HandleLeaveGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		groupId, ok := parseGroupID(c)
		if !ok {
			return
		}
		if err := ctrl.leaveGroupService().Leave(c.Request.Context(), user.ID, groupId); err != nil {
			mapGroupError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
