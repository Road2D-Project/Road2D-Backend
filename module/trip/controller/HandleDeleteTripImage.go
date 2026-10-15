package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleDeleteTripImage godoc
// @Summary      Delete trip image
// @Description  Leader or admin. Removes the stored image from Storage, then clears imageUrl.
// @Tags         trips
// @Security     BearerAuth
// @Param        tripId  path  string  true  "Trip UUID"  format(uuid)
// @Success      204     "Deleted"
// @Failure      401     {object}  share.ErrorResponse
// @Failure      403     {object}  share.ErrorResponse
// @Failure      404     {object}  share.ErrorResponse
// @Router       /trips/{tripId}/image [delete]
func (ctrl *TripController) HandleDeleteTripImage() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		if err := ctrl.tripImages().Delete(c.Request.Context(), tripID.String()); err != nil {
			mapImageError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
