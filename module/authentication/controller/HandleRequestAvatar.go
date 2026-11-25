package controller

import (
	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/authentication/model/response"
	"Road-To-Destination-BE/module/authentication/repository"
	"Road-To-Destination-BE/module/objectStorage"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HandleRequestAvatar godoc
// @Summary      Sign avatar upload
// @Description  Returns a private R2 upload URL and required headers. PUT the raw image bytes, then confirm with path. PNG, JPEG, and WebP only; default maximum 5 MB.
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Param        contentType  query     string  true  "Image content type"
// @Param        sizeBytes    query     int64   true  "Exact file size in bytes"
// @Success      200          {object}  response.AvatarUploadResponse
// @Failure      400          {object}  share.ErrorResponse
// @Failure      401          {object}  share.ErrorResponse
// @Failure      413          {object}  share.ErrorResponse
// @Router       /auth/user/avatar [post]
func (ctrl AuthenticationController) HandleRequestAvatar() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := middleware.GetCurrentUser(c)
		if user == nil {
			jsonError(c, http.StatusUnauthorized, "Unauthorized")
			return
		}
		sizeBytes, err := strconv.ParseInt(c.Query("sizeBytes"), 10, 64)
		if err != nil {
			mapImageError(c, objectStorage.ErrInvalidSize)
			return
		}
		signed, err := ctrl.avatarImages().RequestUpload(c.Request.Context(), user.ID.String(), c.Query("contentType"), sizeBytes)
		if err != nil {
			mapImageError(c, err)
			return
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, response.AvatarUploadResponse(signed))
	}
}

func (ctrl AuthenticationController) avatarImages() *objectStorage.ImageService {
	return objectStorage.NewImageService(
		objectStorage.AvatarNamespace,
		ctrl.storage,
		repository.NewUserAvatarRepository(ctrl.db, ctrl.redisClient),
	)
}

func mapImageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, objectStorage.ErrFileTooLarge):
		jsonError(c, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, objectStorage.ErrUnsupportedType),
		errors.Is(err, objectStorage.ErrInvalidSize),
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
