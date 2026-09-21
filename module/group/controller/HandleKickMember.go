package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleKickMember godoc
// @Summary      Kick a group member
// @Description  Owner may kick admin or member. Admin may kick member only. Cannot kick the owner or yourself (use leave). Target must be active. Status becomes kicked.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path  string  true  "Group UUID"  format(uuid)
// @Param        userId   path  string  true  "Member UUID"  format(uuid)
// @Success      204      "Member kicked"
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      403      {object}  share.ErrorResponse
// @Router       /groups/{groupId}/members/{userId} [delete]
func (ctrl *GroupController) HandleKickMember() gin.HandlerFunc {
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
		role, ok := currentGroupRole(c)
		if !ok {
			jsonError(c, http.StatusForbidden, "group role is missing")
			return
		}
		if err := ctrl.kickMemberService().Kick(c.Request.Context(), user.ID, targetID, groupId, role); err != nil {
			mapGroupError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
