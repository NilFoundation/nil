package mpt

import "errors"

var (
	ErrInvalidAction  = errors.New("invalid action")
	ErrInvalidArgSize = errors.New("invalid arg size for batch update")
	ErrListTooBig     = errors.New("list too big")
	errMissingNode    = errors.New("missing node")
)
