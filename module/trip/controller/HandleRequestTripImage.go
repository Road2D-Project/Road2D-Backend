package controller

import (
	"Road-To-Destination-BE/module/objectStorage"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// HandleRequestTripImage godoc
// @Summary      Sign trip image upload
// @Description  Leader or admin. Returns a private R2 upload URL and required headers. PUT the raw image bytes, then confirm with path. PNG, JPEG, and WebP only; default maximum 5 MB.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId       path      string  true  "Trip UUID"  format(uuid)
// @Param        contentType  query     string  true  "Image content type"
// @Param        sizeBytes    query     int64   true  "Exact file size in bytes"
// @Success      200          {object}  response.TripImageUploadResponse
// @Failure      400          {object}  share.ErrorResponse
// @Failure      401          {object}  share.ErrorResponse
// @Failure      403          {object}  share.ErrorResponse
// @Failure      413          {object}  share.ErrorResponse
// @Router       /trips/{tripId}/image [post]
func (ctrl *TripController) HandleRequestTripImage() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		sizeBytes, err := strconv.ParseInt(c.Query("sizeBytes"), 10, 64)
		if err != nil {
			mapImageError(c, objectStorage.ErrInvalidSize)
			return
		}
		signed, err := ctrl.tripImages().RequestUpload(c.Request.Context(), tripID.String(), c.Query("contentType"), sizeBytes)
		if err != nil {
			mapImageError(c, err)
			return
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, response.TripImageUploadResponse(signed))
	}
}

func (ctrl *TripController) tripImages() *objectStorage.ImageService {
	return objectStorage.NewImageService(
		objectStorage.TripNamespace,
		ctrl.storage,
		repository.NewTripImageRepository(ctrl.db),
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
	case errors.Is(err, repository.ErrTripNotFound):
		jsonError(c, http.StatusNotFound, err.Error())
	default:
		jsonError(c, http.StatusInternalServerError, "could not update image")
	}
}
