package controller

import (
	"net/http"

	"Road-To-Destination-BE/module/trip/model/request"

	"github.com/gin-gonic/gin"
)

// HandleUpdateBranch patches the description and color of one branch.
// The draft inbox is a valid target. Its id comes from GET /trips/{tripId}/draft.
// Only a leader or admin can call this, and not after the trip is completed.
//
// @Summary      Update branch metadata
// @Description  Sets description and/or color on one branch, including the hidden draft inbox. Color is a hex code (#RGB or #RRGGBB); an empty string clears it. At least one field is required. Label, stops, split, and merge are unchanged. Leader or admin. A completed trip is rejected.
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tripId    path      string                      true  "Trip id"  format(uuid)
// @Param        branchId  path      string                      true  "Branch id"  format(uuid)
// @Param        body      body      request.UpdateBranchRequest true  "Description and/or color"
// @Success      200       {object}  response.BranchMetadataResponse
// @Failure      400       {object}  share.ErrorResponse
// @Failure      401       {object}  share.ErrorResponse
// @Failure      403       {object}  share.ErrorResponse
// @Failure      404       {object}  share.ErrorResponse
// @Router       /trips/{tripId}/branches/{branchId} [patch]
func (ctrl *TripController) HandleUpdateBranch() gin.HandlerFunc {
	return func(c *gin.Context) {
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		branchID, ok := parseBranchID(c)
		if !ok {
			return
		}
		var req request.UpdateBranchRequest
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
		updated, err := ctrl.tripBranch().UpdateBranch(c.Request.Context(), tripID, branchID, req)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}
