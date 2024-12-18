package tracer

import (
	"errors"
	"fmt"
)

var (
	ErrCantProofGenesisBlock = errors.New("can't prove genesis block")
	ErrTraceNotFinalized     = errors.New("trace logic malformed: previous opcode not finalized")
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
