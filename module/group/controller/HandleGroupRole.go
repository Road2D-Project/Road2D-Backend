package controller

import (
	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	_ GroupMemberRoleCache        = (*repository.GroupMemberStore)(nil)
	_ ActiveGroupMemberRoleReader = (*repository.GroupMemberRepository)(nil)
)

const groupRoleContextKey = "group_role"

// GroupMemberRoleCache is the Redis copy of an active member's role.
type GroupMemberRoleCache interface {
	GetCachedGroupMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID) (enum.GroupRole, error)
	SetCachedGroupMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID, role enum.GroupRole) error
	GroupMemberRoleCacheKey(groupId uuid.UUID, userId uuid.UUID) string
}

// ActiveGroupMemberRoleReader loads owner/admin/member only when status is active.
type ActiveGroupMemberRoleReader interface {
	FindGroupActiveMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID) (enum.GroupRole, error)
}

func (ctrl *GroupController) handleActiveGroupRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		groupId, err := uuid.Parse(c.Param("groupId"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, share.NewError(http.StatusBadRequest, "invalid groupId"))
			return
		}
		user := middleware.GetCurrentUser(c)
		if user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Unauthorized"))
			return
		}

		store := repository.NewGroupMemberStore(ctrl.redisClient)
		role, err := store.GetCachedGroupMemberRole(c.Request.Context(), groupId, user.ID)
		if err == nil {
			c.Set(groupRoleContextKey, role)
			c.Next()
			return
		}

		if err != nil && !errors.Is(err, repository.ErrMemberRoleNotInCache) {
			// Redis is down; still resolve from Postgres.
		}

		repo := repository.NewGroupMemberRepository(ctrl.db)
		role, err = repo.FindGroupActiveMemberRole(c.Request.Context(), groupId, user.ID)
		if errors.Is(err, repository.ErrUserNotGroupMember) {
			c.AbortWithStatusJSON(http.StatusForbidden, share.NewError(http.StatusForbidden, err.Error()))
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, share.NewError(http.StatusInternalServerError, err.Error()))
			return
		}
		_ = store.SetCachedGroupMemberRole(c.Request.Context(), groupId, user.ID, role)
		c.Set(groupRoleContextKey, role)
		c.Next()
	}
}

func (ctrl *GroupController) requireGroupRole(allowedRoles ...enum.GroupRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get(groupRoleContextKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, share.NewError(http.StatusForbidden, "group role is missing"))
			return
		}
		userRole, ok := roleVal.(enum.GroupRole)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, share.NewError(http.StatusForbidden, "group role is missing"))
			return
		}
		for _, allowedRole := range allowedRoles {
			if userRole == allowedRole {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, share.NewError(http.StatusForbidden, "insufficient group role"))
	}
}

func currentGroupRole(c *gin.Context) (enum.GroupRole, bool) {
	roleVal, exists := c.Get(groupRoleContextKey)
	if !exists {
		return 0, false
	}
	role, ok := roleVal.(enum.GroupRole)
	return role, ok
}
