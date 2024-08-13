package jsonrpc

import "errors"

var (
	errNotImplemented  = errors.New("not implemented")
	errInvalidChainId  = errors.New("invalid ChainId")
	errBlockNotFound   = errors.New("block not found")
	ErrFromAccNotFound = errors.New("\"from\" account not found")
	ErrShardNotFound   = errors.New("shard not found")
	ErrInvalidMessage  = errors.New("invalid message")
)
