package l1

import (
	"github.com/NilFoundation/nil/nil/common"
)

type Event struct {
	Hash        common.Hash `json:"eventHash"`
	BlockNumber uint64      `json:"blkNum"`

	// TODO add json-tagged event received from the L1 Bridge Messenger contract
}

type ProcessedBlock struct {
	Hash        common.Hash `json:"blkHash"`
	BlockNumber uint64      `json:"blkNum"`
	// TODO add all needed fields needed for last processed block info storage
}
