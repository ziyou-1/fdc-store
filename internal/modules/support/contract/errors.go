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
	ErrCreateFailed         = errors.New("support create failed")
	ErrUpdateFailed         = errors.New("support update failed")
)
