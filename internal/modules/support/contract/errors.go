package contract

import "errors"

var (
	ErrGuestForbidden       = errors.New("support guest forbidden")
	ErrConversationNotFound = errors.New("support conversation not found")
	ErrConversationClosed   = errors.New("support conversation closed")
	ErrMessageEmpty         = errors.New("support message empty")
	ErrTranslateFailed      = errors.New("support translate failed")
	ErrContentTypeInvalid   = errors.New("support content type invalid")
	ErrMediaURLRequired     = errors.New("support media url required")
	ErrQuickReplyInvalid    = errors.New("support quick reply invalid")
	ErrQuickReplyNotFound   = errors.New("support quick reply not found")
	ErrMessageNotFound      = errors.New("support message not found")
	// ErrRevokeNotAllowed 覆盖三种情况：不是自己发的、已经撤过、超出时限。
	// 刻意不细分：对外区分「这条不是你发的」和「已经被撤了」等于泄露别人的会话状态。
	ErrRevokeNotAllowed = errors.New("support revoke not allowed")
	ErrCreateFailed     = errors.New("support create failed")
	ErrUpdateFailed     = errors.New("support update failed")
)
