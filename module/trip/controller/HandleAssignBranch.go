package controller

import (
	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/utils/enum"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

var NoPermissionAssignBranch = errors.New("Only admin or leader can asssign another user with branch")

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
		role, _ := c.Get(tripRoleContextKey)
		requestingUserID := middleware.GetCurrentUser(c).ID
		// chỉ có chính họ và admin mới có thể phân công
		if role.(enum.TripRole) == enum.TripRoleLeader || role.(enum.TripRole) == enum.TripRoleAdmin || requestingUserID == *assignRequest.UserID {
			err := ctrl.tripMemberService().AssignBranch(c.Request.Context(), tripID, *assignRequest.UserID, *assignRequest.BranchID)
			if err != nil {
				mapTripError(c, err)
				return
			}
			c.Status(http.StatusNoContent)

		}
		mapTripError(c, NoPermissionAssignBranch)

	}
}
