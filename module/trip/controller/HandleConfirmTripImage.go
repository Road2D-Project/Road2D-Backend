package controller

import (
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleConfirmTripImage godoc
// @Summary      Confirm trip image upload
// @Description  Leader or admin. Stores the public image URL after the file is in Storage. path is the object key returned by the sign call.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tripId  path      string                          true  "Trip UUID"  format(uuid)
// @Param        body    body      request.ConfirmTripImageRequest  true  "Object key"
// @Success      200     {object}  response.TripImageURLResponse
// @Failure      400     {object}  share.ErrorResponse
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Failure      409     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/image/confirm [post]
func (ctrl *TripController) HandleConfirmTripImage() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		var req request.ConfirmTripImageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonBindError(c, err)
			return
		}
		publicURL, err := ctrl.tripImages().Confirm(c.Request.Context(), tripID.String(), req.Path)
		if err != nil {
			mapImageError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.TripImageURLResponse{URL: publicURL})
	}
}
