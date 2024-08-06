package jsonrpc

import "errors"

var (
	errNotImplemented  = errors.New("not implemented")
	errInvalidChainId  = errors.New("invalid ChainId")
	errBlockNotFound   = errors.New("block not found")
	ErrFromAccNotFound = errors.New("\"from\" account not found")
)
