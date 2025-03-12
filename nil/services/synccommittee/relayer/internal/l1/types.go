package l1

import "github.com/ethereum/go-ethereum/common"

type Event struct {
	Hash        common.Hash `json:"eventHash"`
	BlockNumber uint64      `json:"blkNum"`
	BlockHash   common.Hash `json:"blkHash"`

	// Used for proper ordering events while sending to L2
	// Assigned locally (and sequentially for each fetched from the L1 event)
	// Does not guarantee order for events collected by different relayer instances
	SequenceNumber uint64 `json:"sequenceNumber"`

	// TODO add json-tagged event data received from the L1 Bridge Messenger contract
}

type ProcessedBlock struct {
	BlockHash   common.Hash `json:"blkHash"`
	BlockNumber uint64      `json:"blkNum"`
	// TODO add all needed fields needed for last processed block info storage
}
