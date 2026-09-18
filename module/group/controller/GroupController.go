package controller

import (
	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/model/request"
	"Road-To-Destination-BE/module/group/model/response"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/utils/enum"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var _ share.RouterRegistrar = (*GroupController)(nil)

var (
	_ = model.Group{}
	_ = model.GroupMember{}
	_ = request.CreateGroupRequest{}
	_ = request.UpdateGroupRequest{}
	_ = response.GroupResponse{}
	_ = response.GroupListResponse{}
	_ = share.ErrorResponse{}
)

type GroupController struct {
	db          *gorm.DB
	redisClient *redis.Client
	validator   *validator.Validate
	auth        *middleware.AuthenticationMiddleware
}

func NewGroupController(db *gorm.DB, redisClient *redis.Client, validator *validator.Validate, auth *middleware.AuthenticationMiddleware) *GroupController {
	return &GroupController{db: db, redisClient: redisClient, validator: validator, auth: auth}
}

func (ctrl *GroupController) RegisterRoutes(router *gin.RouterGroup) {
	groups := router.Group("/groups", ctrl.auth.RequireAuth())
	{
		groups.POST("", ctrl.HandleCreateGroup())
		groups.GET("", ctrl.HandleListGroups())
		groups.GET("/:groupId", ctrl.HandleGetGroup())
		groups.PATCH("/:groupId", ctrl.handleActiveGroupRole(), ctrl.requireGroupRole(enum.GroupRoleOwner, enum.GroupRoleAdmin), ctrl.HandleUpdateGroup())
		groups.DELETE("/:groupId", ctrl.handleActiveGroupRole(), ctrl.requireGroupRole(enum.GroupRoleOwner), ctrl.HandleDeleteGroup())
	}
}
