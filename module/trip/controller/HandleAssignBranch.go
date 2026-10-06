package controller

import (
	"Road-To-Destination-BE/module/trip/model/request"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (ctrl *TripController) HandleAssignBranch() gin.HandlerFunc {
	return func(c *gin.Context) {
		tripID, ok := parseTripID(c)
		if !ok {
			return
		}
		var assignRequest request.AssignBranchRequest
		if err := c.ShouldBindJSON(&assignRequest); err != nil {
			jsonBindError(c, err)
			return
		}
		if assignRequest.UserID == nil || assignRequest.BranchID == nil {
			jsonError(c, http.StatusBadRequest, "userId and branchId are required")
			return
		}
		err := ctrl.tripMemberService().AssignBranch(c.Request.Context(), tripID, *assignRequest.UserID, *assignRequest.BranchID)
		if err != nil {
			mapTripError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
