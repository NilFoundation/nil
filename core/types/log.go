package types

import (
	"github.com/NilFoundation/nil/common"
)

type Log struct {
	// Consensus fields:
	// address of the contract that generated the event
	Address common.Address
	// list of topics provided by the contract.
	Topics []common.Hash
	// supplied by the contract, usually ABI-encoded
	Data []byte
}
