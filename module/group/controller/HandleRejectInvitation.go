package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleRejectInvitation godoc
// @Summary      Reject a group invitation
// @Description  The invited caller declines. Returns the updated membership with status rejected, not only an error body.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path      string  true  "Group UUID"  format(uuid)
// @Success      200      {object}  response.InvitationResponse
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      404      {object}  share.ErrorResponse
// @Failure      409      {object}  share.ErrorResponse
// @Router       /groups/{groupId}/invitations/reject [post]
func (ctrl *GroupController) HandleRejectInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		groupId, ok := parseGroupID(c)
		if !ok {
			return
		}
		rejected, err := ctrl.invitationRespondingService().RejectInvitation(c.Request.Context(), user.ID, groupId)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, rejected)
	}
}
