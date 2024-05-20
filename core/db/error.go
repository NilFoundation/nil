package db

import "errors"

var (
	ErrKeyNotFound    = errors.New("key not found in db")
	ErrNotImplemented = errors.New("not implemented")
	ErrIteratorCreate = errors.New("failed to create iterator")
)
