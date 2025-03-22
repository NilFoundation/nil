package storage

import "errors"

var (
	ErrKeyExists           = errors.New("object is already stored into the database")
	ErrSerializationFailed = errors.New("failed to (de)serialize object")
)
