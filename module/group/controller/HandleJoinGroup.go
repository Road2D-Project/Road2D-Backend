package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleJoinGroup godoc
// @Summary      Request to join a group
// @Description  The caller creates a pending join request. Owner/admin later accept or reject it. Reuses a left/rejected/kicked membership row.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path      string  true  "Group UUID"  format(uuid)
// @Success      201      {object}  response.InvitationResponse
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      404      {object}  share.ErrorResponse
// @Failure      409      {object}  share.ErrorResponse
// @Router       /groups/{groupId}/join [post]
func (ctrl *GroupController) HandleJoinGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		groupId, ok := parseGroupID(c)
		if !ok {
			return
		}
		created, err := ctrl.joinRequestService().RequestJoin(c.Request.Context(), user.ID, groupId)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusCreated, created)
	}
}
