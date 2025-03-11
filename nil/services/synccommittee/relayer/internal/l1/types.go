package l1

import (
	"math/big"

	"github.com/NilFoundation/nil/nil/common"
)

type EventID string

type Event struct {
	ID EventID `json:"eventId"`
	// TODO add json-tagged event received from the L1 Bridge Messenger contract
}

type ProcessedBlock struct {
	Hash        common.Hash `json:"blkHash"`
	BlockNumber *big.Int    `json:"blkNum"`
	// TODO add all needed fields needed for last processed block info storage
}
