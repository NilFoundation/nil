package jsonrpc

import "errors"

var (
	errNotImplemented = errors.New("not implemented")
	errNotFound       = errors.New("not found")
	errInvalidChainId = errors.New("invalid ChainId")
)
