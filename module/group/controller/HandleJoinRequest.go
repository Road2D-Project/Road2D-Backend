package controller

import (
	"Road-To-Destination-BE/module/group/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleJoinRequest godoc
// @Summary      Accept or reject a join request
// @Description  Owner or admin responds with ?action=accept|reject. Accept makes the user an active member; reject sets status rejected.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path      string  true  "Group UUID"  format(uuid)
// @Param        userId   path      string  true  "Applicant UUID"  format(uuid)
// @Param        action   query     string  true  "accept or reject"  Enums(accept, reject)
// @Success      200      {object}  response.InvitationResponse
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      403      {object}  share.ErrorResponse
// @Failure      404      {object}  share.ErrorResponse
// @Failure      409      {object}  share.ErrorResponse
// @Router       /groups/{groupId}/join-requests/{userId} [post]
func (ctrl *GroupController) HandleJoinRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		groupId, ok := parseGroupID(c)
		if !ok {
			return
		}
		userId, err := parsePathUUID(c, "userId", "invalid userId")
		if err != nil {
			return
		}
		action, ok := parseAcceptOrReject(c)
		if !ok {
			return
		}
		svc := ctrl.joinRequestService()
		var (
			updated *response.InvitationResponse
			svcErr  error
		)
		if action == membershipActionAccept {
			updated, svcErr = svc.AcceptJoinRequest(c.Request.Context(), userId, groupId)
		} else {
			updated, svcErr = svc.RejectJoinRequest(c.Request.Context(), userId, groupId)
		}
		if svcErr != nil {
			mapGroupError(c, svcErr)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
