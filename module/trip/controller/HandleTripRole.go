package controller

import (
	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const tripRoleContextKey = "trip_role"

func (ctrl *TripController) handleActiveTripRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		tripId, err := uuid.Parse(c.Param("tripId"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "invalid tripId"))
			return
		}
		user := middleware.GetCurrentUser(c)
		if user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Unauthorized"))
			return
		}

		store := repository.NewTripMemberStore(ctrl.redisClient)
		role, err := store.GetCachedTripMemberRole(c.Request.Context(), tripId, user.ID)
		if err == nil {
			c.Set(tripRoleContextKey, role)
			c.Next()
			return
		}

		repo := repository.NewTripMemberRepository(ctrl.db)
		role, err = repo.FindTripActiveMemberRole(c.Request.Context(), tripId, user.ID)
		if errors.Is(err, repository.ErrUserNotTripMember) {
			c.AbortWithStatusJSON(http.StatusForbidden, share.NewError(http.StatusForbidden, err.Error()))
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, share.NewError(http.StatusInternalServerError, err.Error()))
			return
		}
		_ = store.SetCachedTripMemberRole(c.Request.Context(), tripId, user.ID, role)
		c.Set(tripRoleContextKey, role)
		c.Next()
	}
}

func (ctrl *TripController) requireTripRole(allowedRoles ...enum.TripRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get(tripRoleContextKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, share.NewError(http.StatusForbidden, "trip role is missing"))
			return
		}
		userRole, ok := roleVal.(enum.TripRole)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, share.NewError(http.StatusForbidden, "trip role is missing"))
			return
		}
		for _, allowedRole := range allowedRoles {
			if userRole == allowedRole {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, share.NewError(http.StatusForbidden, "insufficient trip role"))
	}
}

func currentTripRole(c *gin.Context) (enum.TripRole, bool) {
	roleVal, exists := c.Get(tripRoleContextKey)
	if !exists {
		return 0, false
	}
	role, ok := roleVal.(enum.TripRole)
	return role, ok
}
