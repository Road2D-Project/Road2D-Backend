package controller

import (
	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/objectStorage"

	"github.com/gin-gonic/gin"
)

func (ctrl AuthenticationController) resolveAvatarURL(c *gin.Context, user *model.User) error {
	user.AvatarURL = nil
	if user.AvatarPath == nil || *user.AvatarPath == "" {
		return nil
	}
	if err := objectStorage.ValidateOwnedPath(user.ID.String(), *user.AvatarPath); err != nil {
		return err
	}
	url, err := ctrl.storage.DownloadURL(c.Request.Context(), objectStorage.AvatarNamespace, *user.AvatarPath)
	if err != nil {
		return err
	}
	user.AvatarURL = &url
	return nil
}
