package tracer

import (
	"errors"
)

var (
	ErrCantProofGenesisBlock   = errors.New("can't prove genesis block")
	ErrTraceNotFinalized       = errors.New("trace logic malformed: previous opcode not finalized")
	ErrTracedBlockHashMismatch = errors.New("generated traced block and fetched block hashes are not equal")
	ErrClientReturnedNilBlock  = errors.New("client returned nil block")
	ErrBlocksNotSequential     = errors.New("blocks being traced are not sequential")
)
