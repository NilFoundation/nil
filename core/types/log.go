package types

import (
	"github.com/NilFoundation/nil/common"
)

type Logs []*Log

type Log struct {
	// Address of the contract that generated the event
	Address Address `json:"address"`
	// List of topics provided by the contract
	Topics []common.Hash `json:"topics" ssz-max:"1000"`
	// Supplied by the contract, usually ABI-encoded
	Data []byte `json:"data" ssz-max:"6000"`

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber uint64 `json:"blockNumber"`
}

func NewLog(address Address, data []byte, blockNumber uint64, topics []common.Hash) *Log {
	return &Log{
		Address:     address,
		Topics:      topics,
		Data:        data,
		BlockNumber: blockNumber,
	}
}

func (l *Log) TopicsNum() int {
	return len(l.Topics)
}
