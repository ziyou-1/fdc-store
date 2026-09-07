package supporthttp

import (
	"errors"
	"mime/multipart"
	"strconv"
	"strings"

	supportcontract "github.com/dujiao-next/internal/modules/support/contract"
	supportdomain "github.com/dujiao-next/internal/modules/support/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// SupportService 是买家端客服接口所需的最小端口。
type SupportService interface {
	EnsureConversation(userID uint) (*supportdomain.Conversation, error)
	SendUserMessage(userID uint, input supportcontract.SendMessageInput) (*supportdomain.Message, error)
	ListUserMessages(userID uint, afterID uint, page, pageSize int) (*supportdomain.Conversation, []supportdomain.Message, int64, error)
	MarkReadByUser(userID uint) error
	ListQuickReplies(activeOnly bool) ([]supportdomain.QuickReply, error)
}

// AttachmentUploader 保存买家发送的图片/视频。
// 用独立端口而不是直接依赖上传模块：客服附件的白名单与体积上限
// 与后台素材库不同（要放开视频），必须是另一套策略。
type AttachmentUploader interface {
	SaveSupportAttachment(file *multipart.FileHeader) (url string, mimeType string, err error)
}

// UserHandler 处理买家客服请求。
type UserHandler struct {
	support  SupportService
	uploader AttachmentUploader
}

// NewUserHandler 构造买家客服处理器。
func NewUserHandler(support SupportService, uploader AttachmentUploader) *UserHandler {
	if support == nil {
		panic("support user handler: required dependency is nil")
	}
	return &UserHandler{support: support, uploader: uploader}
}

type sendMessageRequest struct {
	ContentType string `json:"content_type"`
	Content     string `json:"content" binding:"max=4000"`
	MediaURL    string `json:"media_url" binding:"max=512"`
}

// GetConversation 返回买家自己的会话（不存在则创建）。
func (h *UserHandler) GetConversation(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	conversation, err := h.support.EnsureConversation(uid)
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, conversation)
}

// ListMessages 拉取消息。传 after_id 时走增量模式，供前端轮询。
func (h *UserHandler) ListMessages(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	afterID := parseUintQuery(c, "after_id")
	page, pageSize := ginutil.ParsePagination(c)
	conversation, messages, total, err := h.support.ListUserMessages(uid, afterID, page, pageSize)
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

// SendMessage 买家发送一条消息。
func (h *UserHandler) SendMessage(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var request sendMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	message, err := h.support.SendUserMessage(uid, supportcontract.SendMessageInput{
		ContentType: request.ContentType,
		Content:     request.Content,
		MediaURL:    request.MediaURL,
	})
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, message)
}

// MarkRead 买家已读。
func (h *UserHandler) MarkRead(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	if err := h.support.MarkReadByUser(uid); err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// UploadAttachment 买家上传图片或视频，返回可直接放进消息的 media_url。
func (h *UserHandler) UploadAttachment(c *gin.Context) {
	if _, ok := ginutil.GetUserID(c); !ok {
		return
	}
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

// ListQuickReplies 返回启用中的常见问题，买家端渲染成快捷按钮。
func (h *UserHandler) ListQuickReplies(c *gin.Context) {
	replies, err := h.support.ListQuickReplies(true)
	if err != nil {
		respondSupportError(c, err)
		return
	}
	response.Success(c, replies)
}

func respondSupportError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, supportcontract.ErrGuestForbidden):
		ginutil.RespondError(c, response.CodeBadRequest, "error.support_login_required", err)
	case errors.Is(err, supportcontract.ErrConversationNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.support_conversation_not_found", err)
	case errors.Is(err, supportcontract.ErrMessageEmpty):
		ginutil.RespondError(c, response.CodeBadRequest, "error.support_message_empty", err)
	case errors.Is(err, supportcontract.ErrTranslateFailed):
		ginutil.RespondError(c, response.CodeInternal, "error.support_translate_failed", err)
	case errors.Is(err, supportcontract.ErrMediaURLRequired),
		errors.Is(err, supportcontract.ErrContentTypeInvalid),
		errors.Is(err, supportcontract.ErrQuickReplyInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
	case errors.Is(err, supportcontract.ErrQuickReplyNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.not_found", err)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.bad_request", err)
	}
}

func parseUintQuery(c *gin.Context, key string) uint {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return uint(value)
}
