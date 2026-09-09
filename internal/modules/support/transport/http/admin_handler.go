package supporthttp

import (
	"strings"

	supportcontract "github.com/dujiao-next/internal/modules/support/contract"
	supportdomain "github.com/dujiao-next/internal/modules/support/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminSupportService 是客服工作台所需的最小端口。
type AdminSupportService interface {
	ListConversations(filter supportcontract.ConversationListFilter) ([]supportcontract.ConversationView, int64, error)
	CountUnreadConversations() (int64, error)
	ListAgentMessages(conversationID, afterID uint, page, pageSize int) (
		*supportdomain.Conversation, []supportdomain.Message, int64, error)
	SendAgentMessage(adminID, conversationID uint, input supportcontract.SendMessageInput) (*supportdomain.Message, error)
	RevokeAgentMessage(messageID, agentID uint) (*supportdomain.Message, error)
	MarkReadByAgent(conversationID uint) error
	SetConversationStatus(conversationID uint, status string) (*supportdomain.Conversation, error)
	ListQuickReplies(activeOnly bool) ([]supportdomain.QuickReply, error)
	CreateQuickReply(input supportcontract.QuickReplyInput) (*supportdomain.QuickReply, error)
	UpdateQuickReply(id uint, input supportcontract.QuickReplyInput) (*supportdomain.QuickReply, error)
	DeleteQuickReply(id uint) error
}

// AdminHandler 处理客服工作台请求。
type AdminHandler struct {
	support  AdminSupportService
	uploader AttachmentUploader
}

// NewAdminHandler 构造客服工作台处理器。
func NewAdminHandler(support AdminSupportService, uploader AttachmentUploader) *AdminHandler {
	if support == nil {
		panic("support admin handler: required dependency is nil")
	}
	return &AdminHandler{support: support, uploader: uploader}
}

type agentMessageRequest struct {
	ContentType string `json:"content_type"`
	Content     string `json:"content" binding:"max=4000"`
	MediaURL    string `json:"media_url" binding:"max=512"`
	// ReplyToID 引用的消息 ID，0 表示普通发送。
	// 服务层会校验它确实属于同一会话，跨会话引用会被静默丢弃而不是报错——
	// 报错等于告诉客服「那条消息存在但不是这个会话的」，没必要泄露。
	ReplyToID uint `json:"reply_to_id"`
}

type conversationStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=open closed"`
}

type quickReplyRequest struct {
	Title     string `json:"title" binding:"required,max=120"`
	Content   string `json:"content" binding:"max=4000"`
	IsActive  bool   `json:"is_active"`
	SortOrder int    `json:"sort_order"`
}

// ListConversations 会话列表，未读优先。
func (h *AdminHandler) ListConversations(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	unreadOnly, err := ginutil.ParseQueryBool(c, "unread_only")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	userID, err := ginutil.ParseQueryUint(c.Query("user_id"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	views, total, err := h.support.ListConversations(supportcontract.ConversationListFilter{
		Page:       page,
		PageSize:   pageSize,
		Status:     c.Query("status"),
		UnreadOnly: unreadOnly,
		UserID:     userID,
	})
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.SuccessWithPage(c, views, response.BuildPagination(page, pageSize, total))
}

// CountUnread 工作台角标：待处理会话数。
func (h *AdminHandler) CountUnread(c *gin.Context) {
	total, err := h.support.CountUnreadConversations()
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, gin.H{"unread_conversations": total})
}

// ListMessages 客服查看某会话消息，支持 after_id 增量轮询。
func (h *AdminHandler) ListMessages(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	conversation, messages, total, err := h.support.ListAgentMessages(id, parseUintQuery(c, "after_id"), page, pageSize)
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, gin.H{
		"conversation": conversation,
		"messages":     messages,
		"total":        total,
	})
}

// SendMessage 客服回复。
func (h *AdminHandler) SendMessage(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var request agentMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	message, err := h.support.SendAgentMessage(adminID, id, supportcontract.SendMessageInput{
		ContentType: request.ContentType,
		Content:     request.Content,
		MediaURL:    request.MediaURL,
		ReplyToID:   request.ReplyToID,
	})
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, message)
}

// RevokeMessage 客服撤回自己刚发的消息。
// 只标记不删除：support_messages 是只追加表，前端把撤回态渲染成「已撤回」。
func (h *AdminHandler) RevokeMessage(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	adminID, _ := ginutil.GetAdminID(c)
	message, err := h.support.RevokeAgentMessage(id, adminID)
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, message)
}

// UploadAttachment 客服上传图片或视频，走独立的 support 上传策略。
func (h *AdminHandler) UploadAttachment(c *gin.Context) {
	if h.uploader == nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.support_attachment_disabled", nil)
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	url, mimeType, err := h.uploader.SaveSupportAttachment(file)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.support_attachment_rejected", err)
		return
	}
	contentType := supportdomain.ContentTypeImage
	if strings.HasPrefix(strings.ToLower(mimeType), "video/") {
		contentType = supportdomain.ContentTypeVideo
	}
	response.Success(c, gin.H{
		"media_url":    url,
		"mime_type":    mimeType,
		"content_type": contentType,
	})
}

// MarkRead 客服已读。
func (h *AdminHandler) MarkRead(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.support.MarkReadByAgent(id); err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// UpdateStatus 客服开关会话。
func (h *AdminHandler) UpdateStatus(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var request conversationStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	conversation, err := h.support.SetConversationStatus(id, request.Status)
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, conversation)
}

// ListQuickReplies 后台常见问题列表，含停用项。
func (h *AdminHandler) ListQuickReplies(c *gin.Context) {
	replies, err := h.support.ListQuickReplies(false)
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, replies)
}

// CreateQuickReply 新增常见问题。
func (h *AdminHandler) CreateQuickReply(c *gin.Context) {
	var request quickReplyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	reply, err := h.support.CreateQuickReply(supportcontract.QuickReplyInput{
		Title:     request.Title,
		Content:   request.Content,
		IsActive:  request.IsActive,
		SortOrder: request.SortOrder,
	})
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, reply)
}

// UpdateQuickReply 修改常见问题。
func (h *AdminHandler) UpdateQuickReply(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var request quickReplyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	reply, err := h.support.UpdateQuickReply(id, supportcontract.QuickReplyInput{
		Title:     request.Title,
		Content:   request.Content,
		IsActive:  request.IsActive,
		SortOrder: request.SortOrder,
	})
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, reply)
}

// DeleteQuickReply 删除常见问题。
func (h *AdminHandler) DeleteQuickReply(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.support.DeleteQuickReply(id); err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}
