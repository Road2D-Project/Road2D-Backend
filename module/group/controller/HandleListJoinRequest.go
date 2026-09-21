package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleListJoinRequest godoc
// @Summary      List pending join requests
// @Description  Owner or admin lists users waiting to join this group.
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        groupId  path      string  true  "Group UUID"  format(uuid)
// @Success      200      {object}  response.JoinRequestListResponse
// @Failure      400      {object}  share.ErrorResponse
// @Failure      401      {object}  share.ErrorResponse
// @Failure      403      {object}  share.ErrorResponse
// @Router       /groups/{groupId}/join-requests [get]
func (ctrl *GroupController) HandleListJoinRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		groupId, ok := parseGroupID(c)
		if !ok {
			return
		}
		list, err := ctrl.joinRequestService().ListJoinRequests(c.Request.Context(), groupId)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
