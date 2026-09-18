package controller

import (
	"Road-To-Destination-BE/module/group/model/request"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleUpdateGroup godoc
// @Summary      Update group
// @Description  Owner or admin may change name and/or description. Members cannot update group info. Send only the fields to change.
// @Tags         groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path      string                   true  "Group UUID"  format(uuid)
// @Param        body     body      request.UpdateGroupRequest  true  "Fields to update"
// @Success      200      {object}  response.GroupResponse
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      403      {object}  share.ErrorResponse
// @Failure      404      {object}  share.ErrorResponse
// @Router       /groups/{groupId} [patch]
func (ctrl *GroupController) HandleUpdateGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		groupID, ok := parseGroupID(c)
		if !ok {
			return
		}
		role, ok := currentGroupRole(c)
		if !ok {
			jsonError(c, http.StatusForbidden, "group role is missing")
			return
		}
		var req request.UpdateGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonBindError(c, err)
			return
		}
		updated, err := ctrl.groupService().UpdateGroup(c.Request.Context(), groupID, req, role)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
