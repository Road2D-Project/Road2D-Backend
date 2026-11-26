package controller

import (
	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/conversation/model/response"
	"Road-To-Destination-BE/module/conversation/repository"
	"Road-To-Destination-BE/module/conversation/service"
	"Road-To-Destination-BE/module/share"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var _ share.RouterRegistrar = (*ChatController)(nil)

var (
	_ = response.ConversationListResponse{}
	_ = response.ConversationResponse{}
	_ = response.MessageListResponse{}
	_ = response.MessageResponse{}
	_ = response.EventListResponse{}
	_ = response.ReplayRequiredResponse{}
)

type ChatController struct {
	reads       *service.ConversationService
	messages    *service.MessageService
	state       *service.ConversationStateService
	events      *service.EventReplayService
	auth        *middleware.AuthenticationMiddleware
	sendLimiter MessageSendLimiter
}

type MessageSendLimiter interface{ RequireLimiter() gin.HandlerFunc }

func NewChatController(db *gorm.DB, auth *middleware.AuthenticationMiddleware, sendLimiter MessageSendLimiter) *ChatController {
	reads := repository.NewConversationRepository(db)
	mutations := repository.NewMutationRepository(db)
	transaction := func(ctx context.Context, actor, id uuid.UUID, run func(service.ChatWrite) error) error {
		return mutations.Transaction(ctx, actor, id, func(tx *repository.MutationRepository) error {
			return run(service.ChatWrite{Conversation: tx.Conversation, Member: tx.Member, Now: tx.Now, Messages: tx, State: tx, Events: tx})
		})
	}
	return &ChatController{reads: service.NewConversationService(reads), messages: service.NewMessageService(transaction),
		state: service.NewConversationStateService(transaction), events: service.NewEventReplayService(reads), auth: auth, sendLimiter: sendLimiter}
}

func (ctrl *ChatController) RegisterRoutes(router *gin.RouterGroup) {
	chat := router.Group("/conversations", ctrl.auth.RequireAuth())
	chat.GET("", ctrl.HandleListConversations())
	chat.GET("/:conversationId", ctrl.HandleGetConversation())
	chat.GET("/:conversationId/messages", ctrl.HandleListMessages())
	if ctrl.sendLimiter != nil {
		chat.POST("/:conversationId/messages", ctrl.sendLimiter.RequireLimiter(), ctrl.HandleCreateMessage())
	} else {
		chat.POST("/:conversationId/messages", ctrl.HandleCreateMessage())
	}
	chat.PATCH("/:conversationId/messages/:messageId", ctrl.HandleUpdateMessage())
	chat.DELETE("/:conversationId/messages/:messageId", ctrl.HandleDeleteMessage())
	chat.PUT("/:conversationId/read", ctrl.HandleMarkRead())
	chat.PATCH("/:conversationId/preferences", ctrl.HandleUpdatePreferences())
	chat.GET("/:conversationId/events", ctrl.HandleListEvents())
}

// Assert transaction-bound adapters satisfy only the capabilities consumers use.
var (
	_ service.MessageStore              = (*repository.MutationRepository)(nil)
	_ service.UserStateStore            = (*repository.MutationRepository)(nil)
	_ service.ConversationEventAppender = (*repository.MutationRepository)(nil)
)
