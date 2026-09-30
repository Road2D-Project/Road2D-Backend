package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/trip/model/request"

	"github.com/gin-gonic/gin"
)

// HandlePreviewLocations reviews the bike route along ordered locations.
// No trip is required and nothing is stored. The body of the response is the
// mainBranch field of create trip.
//
// @Summary      Preview locations
// @Description  Compute the bike route along ordered location ids before a trip exists. Nothing is stored. Pass this response as mainBranch on POST /trips to open the trip on that main branch.
// @Tags         planning
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      request.PreviewLocationsRequest  true  "Ordered location ids"
// @Success      200   {object}  response.PreviewLocationsResponse
// @Failure      400   {object}  share.ErrorResponse
// @Failure      401   {object}  share.ErrorResponse
// @Failure      404   {object}  share.ErrorResponse
// @Failure      429   {object}  share.ErrorResponse
// @Failure      500   {object}  share.ErrorResponse
// @Failure      502   {object}  share.ErrorResponse
// @Router       /planning/preview [post]
func (ctrl *TripController) HandlePreviewLocations() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req request.PreviewLocationsRequest
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
		out, err := ctrl.computeTrip().PreviewLocations(c.Request.Context(), req.LocationIDs)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
	}
}
