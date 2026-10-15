package controller

import (
	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/authentication/model/response"
	"Road-To-Destination-BE/module/authentication/repository"
	"Road-To-Destination-BE/module/objectStorage"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HandleRequestAvatar godoc
// @Summary      Sign avatar upload
// @Description  Returns a signed URL. PUT the image bytes to url, then call confirm with path. contentType is image/png, image/jpeg, or image/webp.
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Param        contentType  query     string  true  "Image content type"
// @Success      200          {object}  response.AvatarUploadResponse
// @Failure      400          {object}  share.ErrorResponse
// @Failure      401          {object}  share.ErrorResponse
// @Router       /auth/user/avatar [post]
func (ctrl AuthenticationController) HandleRequestAvatar() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := middleware.GetCurrentUser(c)
		if user == nil {
			jsonError(c, http.StatusUnauthorized, "Unauthorized")
			return
		}
		signed, err := ctrl.avatarImages().RequestUpload(c.Request.Context(), user.ID.String(), c.Query("contentType"))
		if err != nil {
			mapImageError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.AvatarUploadResponse{URL: signed.URL, Path: signed.Path})
	}
}

func (ctrl AuthenticationController) avatarImages() *objectStorage.ImageService {
	return objectStorage.NewImageService(
		repository.UserAvatarPaths{},
		objectStorage.NewClient(),
		repository.NewUserAvatarRepository(ctrl.db, ctrl.redisClient),
		objectStorage.ImageExt,
	)
}

func mapImageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, objectStorage.ErrUnsupportedType),
		errors.Is(err, objectStorage.ErrInvalidPath),
		errors.Is(err, objectStorage.ErrInvalidObjectPath):
		jsonError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, objectStorage.ErrNotUploaded):
		jsonError(c, http.StatusConflict, err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		jsonError(c, http.StatusNotFound, "user not found")
	default:
		jsonError(c, http.StatusInternalServerError, "could not update image")
	}
}
