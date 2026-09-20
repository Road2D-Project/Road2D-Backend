package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleInviteNewMember godoc
// @Summary      Invite a user to a group
// @Description  Owner or admin invites a user by UUID. Reuses a left/rejected/kicked membership row. The invitee later accepts or rejects.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path      string  true  "Group UUID"  format(uuid)
// @Param        userId   path      string  true  "Invitee UUID"  format(uuid)
// @Success      201      {object}  response.InvitationResponse
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      403      {object}  share.ErrorResponse
// @Failure      404      {object}  share.ErrorResponse
// @Failure      409      {object}  share.ErrorResponse
// @Router       /groups/{groupId}/invite/{userId} [post]
func (ctrl *GroupController) HandleInviteNewMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		groupId, ok := parseGroupID(c)
		if !ok {
			return
		}
		invitedUserId, err := parsePathUUID(c, "userId", "invalid userId")
		if err != nil {
			return
		}
		created, err := ctrl.inviteNewMemberService().Invite(
			c.Request.Context(),
			invitedUserId,
			groupId,
			user.ID,
			user.Username,
		)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusCreated, created)
	}
}
