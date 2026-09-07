package contract

import "errors"

var (
	ErrDisabled            = errors.New("free claim disabled")
	ErrMemberOnly          = errors.New("free claim member only")
	ErrPoolEmpty           = errors.New("free claim pool empty")
	ErrAlreadyClaimed      = errors.New("free claim already claimed today")
	ErrGuestForbidden      = errors.New("free claim forbidden for guest")
	ErrItemMismatch        = errors.New("free claim item mismatch")
	ErrClaimCreateFailed   = errors.New("free claim create failed")
	ErrConfigUpdateFailed  = errors.New("free claim config update failed")
	ErrPoolItemInvalid     = errors.New("free claim pool item invalid")
	ErrTransactionRequired = errors.New("free claim transaction required")
)
