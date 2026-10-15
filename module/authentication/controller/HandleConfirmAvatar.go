package controller

import (
	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/authentication/model/request"
	"Road-To-Destination-BE/module/authentication/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleConfirmAvatar godoc
// @Summary      Confirm avatar upload
// @Description  Stores the public avatar URL after the file is in Storage. path is the object key returned by the sign call.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      request.ConfirmAvatarRequest  true  "Object key"
// @Success      200   {object}  response.AvatarURLResponse
// @Failure      400   {object}  share.ErrorResponse
// @Failure      401   {object}  share.ErrorResponse
// @Failure      409   {object}  share.ErrorResponse
// @Router       /auth/user/avatar/confirm [post]
func (ctrl AuthenticationController) HandleConfirmAvatar() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := middleware.GetCurrentUser(c)
		if user == nil {
			jsonError(c, http.StatusUnauthorized, "Unauthorized")
			return
		}
		var req request.ConfirmAvatarRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonBindError(c, err)
			return
		}
		publicURL, err := ctrl.avatarImages().Confirm(c.Request.Context(), user.ID.String(), req.Path)
		if err != nil {
			mapImageError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.AvatarURLResponse{URL: publicURL})
	}
}
