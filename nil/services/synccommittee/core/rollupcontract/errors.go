package rollupcontract

import "errors"

var (
	ErrBatchAlreadyFinalized = errors.New("batch already finalized")
	ErrBatchAlreadyCommitted = errors.New("batch already committed")
	ErrBatchNotCommitted     = errors.New("batch has not been committed")
	ErrInvalidBatchIndex     = errors.New("batch index is invalid")
	ErrInvalidVersionedHash  = errors.New("versioned hash is invalid")
)
