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

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber uint64
}
