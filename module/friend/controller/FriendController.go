package controller

import (
	"Road-To-Destination-BE/middleware"
	authRepo "Road-To-Destination-BE/module/authentication/repository"
	"Road-To-Destination-BE/module/friend/model/response"
	"Road-To-Destination-BE/module/friend/repository"
	"Road-To-Destination-BE/module/friend/service"
	"Road-To-Destination-BE/module/share"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var _ share.RouterRegistrar = (*FriendController)(nil)

var (
	_ = response.FriendListResponse{}
	_ = response.FriendRequestListResponse{}
	_ = response.FriendRequestResponse{}
	_ = response.FriendSearchResponse{}
	_ = share.ErrorResponse{}
)

type FriendController struct {
	db   *gorm.DB
	auth *middleware.AuthenticationMiddleware
}

func NewFriendController(db *gorm.DB, auth *middleware.AuthenticationMiddleware) *FriendController {
	return &FriendController{db: db, auth: auth}
}

func (ctrl *FriendController) RegisterRoutes(router *gin.RouterGroup) {
	friends := router.Group("/friends", ctrl.auth.RequireAuth())
	{
		friends.GET("", ctrl.HandleListFriends())
		friends.GET("/search", ctrl.HandleSearchUsers())
		friends.GET("/requests", ctrl.HandleListRequests())
		friends.POST("/requests/:userId", ctrl.HandleSendRequest())
		friends.POST("/requests/:userId/respond", ctrl.HandleRespondRequest())
		friends.DELETE("/requests/:userId", ctrl.HandleCancelRequest())
		friends.DELETE("/:userId", ctrl.HandleUnfriend())
	}
}

func (ctrl *FriendController) friendService() *service.FriendService {
	repo := repository.NewFriendshipRepository(ctrl.db)
	return service.NewFriendService(repo, repo, authRepo.NewUserRepository(ctrl.db))
}
