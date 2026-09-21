package controller

import (
	"Road-To-Destination-BE/module/group/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleRespondInvitation godoc
// @Summary      Respond to a group invitation
// @Description  The invited caller accepts or rejects with ?action=accept|reject. Accept makes them an active member; reject sets status rejected.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path      string  true  "Group UUID"  format(uuid)
// @Param        action   query     string  true  "accept or reject"  Enums(accept, reject)
// @Success      200      {object}  response.InvitationResponse
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      404      {object}  share.ErrorResponse
// @Failure      409      {object}  share.ErrorResponse
// @Router       /groups/{groupId}/invitations [post]
func (ctrl *GroupController) HandleRespondInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		groupId, ok := parseGroupID(c)
		if !ok {
			return
		}
		action, ok := parseAcceptOrReject(c)
		if !ok {
			return
		}
		svc := ctrl.invitationRespondingService()
		var (
			updated *response.InvitationResponse
			err     error
		)
		if action == membershipActionAccept {
			updated, err = svc.AcceptInvitation(c.Request.Context(), user.ID, groupId)
		} else {
			updated, err = svc.RejectInvitation(c.Request.Context(), user.ID, groupId)
		}
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
