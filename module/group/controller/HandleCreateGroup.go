package controller

import (
	"Road-To-Destination-BE/module/group/model/request"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleCreateGroup godoc
// @Summary      Create group
// @Description  Create a standing group. The caller becomes owner (active). Optional adminUsernames are added immediately as active admins. Unknown usernames abort the whole create. Roles: owner, admin, member. Membership statuses: invited, pending, active, rejected, left, kicked.
// @Tags         groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      request.CreateGroupRequest  true  "Group name, optional description, optional admin usernames"
// @Success      201   {object}  response.GroupResponse
// @Failure      400   {object}  share.ErrorResponse
// @Failure      401   {object}  share.ErrorResponse
// @Router       /groups [post]
func (ctrl *GroupController) HandleCreateGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUserOrAbort(c)
		if user == nil {
			return
		}
		var req request.CreateGroupRequest
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
		created, err := ctrl.groupService().CreateGroup(c.Request.Context(), user, req)
		if err != nil {
			mapGroupError(c, err)
			return
		}
		c.JSON(http.StatusCreated, created)
	}
}
