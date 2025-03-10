package tracer

import (
	"errors"
	"fmt"
)

var (
	ErrCantProofGenesisBlock   = errors.New("can't prove genesis block")
	ErrTraceNotFinalized       = errors.New("trace logic malformed: previous opcode not finalized")
	ErrTracedBlockHashMismatch = errors.New("generated traced block and fetched block hashes are not equal")
	ErrClientReturnedNilBlock  = errors.New("client returned nil block")
)

type managedTracerFailureError struct {
	underlying error
}

func (e managedTracerFailureError) Unwrap() error {
	return e.underlying
}

func (e managedTracerFailureError) Error() string {
	return fmt.Sprintf("managed tracer failure: %v", e.underlying)
}
