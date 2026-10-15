package controller

import (
	"Road-To-Destination-BE/module/objectStorage"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleRequestTripImage godoc
// @Summary      Sign trip image upload
// @Description  Leader or admin. PUT the image bytes to url, then call confirm with path. contentType is image/png, image/jpeg, or image/webp.
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        tripId       path      string  true  "Trip UUID"  format(uuid)
// @Param        contentType  query     string  true  "Image content type"
// @Success      200          {object}  response.TripImageUploadResponse
// @Failure      400          {object}  share.ErrorResponse
// @Failure      401          {object}  share.ErrorResponse
// @Failure      403          {object}  share.ErrorResponse
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
		signed, err := ctrl.tripImages().RequestUpload(c.Request.Context(), tripID.String(), c.Query("contentType"))
		if err != nil {
			mapImageError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.TripImageUploadResponse{URL: signed.URL, Path: signed.Path})
	}
}

func (ctrl *TripController) tripImages() *objectStorage.ImageService {
	return objectStorage.NewImageService(
		repository.TripImagePaths{},
		objectStorage.NewClient(),
		repository.NewTripImageRepository(ctrl.db),
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
	case errors.Is(err, repository.ErrTripNotFound):
		jsonError(c, http.StatusNotFound, err.Error())
	default:
		jsonError(c, http.StatusInternalServerError, "could not update image")
	}
}
