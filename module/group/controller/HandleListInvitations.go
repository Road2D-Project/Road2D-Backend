package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleListInvitations godoc
// @Summary      List my group invitations
// @Description  Returns invitations waiting for the caller to accept or reject. Other users' invitations are never included.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.InvitationListResponse
// @Failure      401  {object}  share.ErrorResponse
// @Router       /groups/invitations [get]
func (ctrl *GroupController) HandleListInvitations() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		list, err := ctrl.invitationRespondingService().ListInvitations(c.Request.Context(), user.ID)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
