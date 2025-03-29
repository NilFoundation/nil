package storage

import "errors"

var (
	ErrStateRootNotInitialized = errors.New("proved state root is not initialized")
	ErrTaskAlreadyExists       = errors.New("task with a given identifier already exists")
	ErrSerializationFailed     = errors.New("failed to serialize/deserialize object")
	ErrCapacityLimitReached    = errors.New("storage capacity limit reached")
	errNilTaskEntry            = errors.New("task entry cannot be nil")
)
