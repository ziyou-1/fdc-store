package contract

import "errors"

var (
	ErrStationInvalid  = errors.New("token station invalid")
	ErrStationNotFound = errors.New("token station not found")
)
