package controller

import (
	"Road-To-Destination-BE/module/group/model/request"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleUpdateMember godoc
// @Summary      Update my nickname
// @Description  An active member may change only their own nickname. Path userId must match the caller.
// @Tags         groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path      string                      true  "Group UUID"  format(uuid)
// @Param        userId   path      string                      true  "Member UUID"  format(uuid)
// @Param        body     body      request.UpdateMemberRequest  true  "New nickname"
// @Success      200      {object}  response.GroupMemberResponse
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      403      {object}  share.ErrorResponse
// @Router       /groups/{groupId}/members/{userId} [patch]
func (ctrl *GroupController) HandleUpdateMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		groupId, ok := parseGroupID(c)
		if !ok {
			return
		}
		targetID, err := parsePathUUID(c, "userId", "invalid userId")
		if err != nil {
			return
		}
		var req request.UpdateMemberRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonBindError(c, err)
			return
		}
		if ctrl.validator != nil {
			if err := ctrl.validator.Struct(req); err != nil {
				jsonBindError(c, err)
				return
			}
		}
		updated, err := ctrl.groupMemberService().UpdateNickname(c.Request.Context(), user.ID, targetID, groupId, req.Nickname)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
