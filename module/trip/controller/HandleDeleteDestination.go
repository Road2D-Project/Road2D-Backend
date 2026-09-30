package controller

import (
	"net/http"

	"Road-To-Destination-BE/middleware"

	"github.com/gin-gonic/gin"
)

// HandleDeleteDestination deletes a pin that is still editing.
// A pin that sits only on draft branches is removed with the row.
// A pin that sits on a route branch is stripped first and the graph is checked again.
// A locked trip that still uses the pin on a route blocks the delete.
//
// @Summary      Delete destination
// @Description  Deletes a destination that is still editing. A pin that sits only on draft branches is removed immediately. A pin on a route branch is stripped first; the delete is rejected when that would disconnect the graph or when a trip that is no longer planning still uses the pin on a route. Travels that pointed at the pin are removed with the row.
// @Tags         planning
// @Produce      json
// @Security     BearerAuth
// @Param        destinationId  path  string  true  "Destination UUID"  format(uuid)
// @Success      204            "Destination deleted"
// @Failure      400            {object}  share.ErrorResponse
// @Failure      401            {object}  share.ErrorResponse
// @Failure      404            {object}  share.ErrorResponse
// @Failure      409            {object}  share.ErrorResponse
// @Router       /planning/destination/{destinationId} [delete]
func (ctrl *TripController) HandleDeleteDestination() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUserOrAbort(c) == nil {
			return
		}
		destinationID, ok := parseDestinationID(c)
		if !ok {
			return
		}
		affected, err := ctrl.tripBranch().DeleteDestination(c.Request.Context(), destinationID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		// MarkChanged reads the cache keys after the handler returns. This route has no
		// tripId on the path, so the keys come from the trips that just lost the pin.
		if len(affected) > 0 {
			keys := make([]string, 0, len(affected)*2)
			for _, tripID := range affected {
				id := tripID.String()
				keys = append(keys, "resp:graph:"+id, "resp:travels:"+id, "resp:draft:"+id)
			}
			c.Set(middleware.MainKeyController, keys)
		}
		c.Status(http.StatusNoContent)
	}
}
