package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleDeleteGroup godoc
// @Summary      Delete group
// @Description  Owner only. Fails with 409 if the group still has trips (trips.group_id). Members cascade-delete with the group.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path  string  true  "Group UUID"  format(uuid)
// @Success      204      "Group deleted"
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      403      {object}  share.ErrorResponse
// @Failure      404      {object}  share.ErrorResponse
// @Failure      409      {object}  share.ErrorResponse
// @Router       /groups/{groupId} [delete]
func (ctrl *GroupController) HandleDeleteGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		groupID, ok := parseGroupID(c)
		if !ok {
			return
		}
		if err := ctrl.groupService().DeleteGroup(c.Request.Context(), groupID); err != nil {
			mapGroupError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
