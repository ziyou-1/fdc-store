package supporthttp

import "github.com/gin-gonic/gin"

// RegisterUserRoutes 注册买家客服端点。
func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	if user == nil || handler == nil {
		panic("support user routes: required dependency is nil")
	}
	user.GET("/support/conversation", handler.GetConversation)
	user.GET("/support/messages", handler.ListMessages)
	user.POST("/support/messages", handler.SendMessage)
	user.POST("/support/messages/read", handler.MarkRead)
	user.POST("/support/attachments", handler.UploadAttachment)
	user.GET("/support/quick-replies", handler.ListQuickReplies)
}

// RegisterAdminRoutes 注册客服工作台端点。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	if admin == nil || handler == nil {
		panic("support admin routes: required dependency is nil")
	}
	admin.GET("/support/conversations", handler.ListConversations)
	admin.GET("/support/unread-count", handler.CountUnread)
	admin.GET("/support/conversations/:id/messages", handler.ListMessages)
	admin.POST("/support/conversations/:id/messages", handler.SendMessage)
	admin.POST("/support/messages/:id/revoke", handler.RevokeMessage)
	admin.POST("/support/attachments", handler.UploadAttachment)
	admin.POST("/support/conversations/:id/read", handler.MarkRead)
	admin.PUT("/support/conversations/:id/status", handler.UpdateStatus)
	admin.GET("/support/quick-replies", handler.ListQuickReplies)
	admin.POST("/support/quick-replies", handler.CreateQuickReply)
	admin.PUT("/support/quick-replies/:id", handler.UpdateQuickReply)
	admin.DELETE("/support/quick-replies/:id", handler.DeleteQuickReply)
}
