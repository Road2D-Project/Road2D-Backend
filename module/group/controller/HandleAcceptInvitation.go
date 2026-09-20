package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleAcceptInvitation godoc
// @Summary      Accept a group invitation
// @Description  The invited caller becomes an active member. Returns the updated membership, not only an error body.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path      string  true  "Group UUID"  format(uuid)
// @Success      200      {object}  response.InvitationResponse
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      404      {object}  share.ErrorResponse
// @Failure      409      {object}  share.ErrorResponse
// @Router       /groups/{groupId}/invitations/accept [post]
func (ctrl *GroupController) HandleAcceptInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		groupId, ok := parseGroupID(c)
		if !ok {
			return
		}
		accepted, err := ctrl.invitationRespondingService().AcceptInvitation(c.Request.Context(), user.ID, groupId)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, accepted)
	}
}
