package errors

import "errors"

var (
	ErrOldBlock            = errors.New("received old block")
	ErrOutOfOrder          = errors.New("received block is out of order")
	ErrHashMismatch        = errors.New("block hash mismatch")
	ErrInvalidProposedHash = errors.New("invalid prposed hash")
)
