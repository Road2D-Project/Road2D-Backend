package controller

import (
	"Road-To-Destination-BE/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleDeleteAvatar godoc
// @Summary      Delete avatar
// @Description  Removes the stored avatar from Storage, then clears avatarUrl.
// @Tags         auth
// @Security     BearerAuth
// @Success      204  "Deleted"
// @Failure      401  {object}  share.ErrorResponse
// @Router       /auth/user/avatar [delete]
func (ctrl AuthenticationController) HandleDeleteAvatar() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := middleware.GetCurrentUser(c)
		if user == nil {
			jsonError(c, http.StatusUnauthorized, "Unauthorized")
			return
		}
		if err := ctrl.avatarImages().Delete(c.Request.Context(), user.ID.String()); err != nil {
			mapImageError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
