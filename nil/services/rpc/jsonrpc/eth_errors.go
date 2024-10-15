package jsonrpc

import "errors"

var (
	errNotImplemented = errors.New("not implemented")
	ErrToAccNotFound  = errors.New("\"to\" account not found")
	ErrShardNotFound  = errors.New("shard not found")
	ErrInvalidMessage = errors.New("invalid message")
)
