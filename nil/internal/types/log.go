package types

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
)

type Logs []*Log

type Log struct {
	// Address of the contract that generated the event
	Address Address `json:"address"`
	// List of topics provided by the contract
	Topics []common.Hash `json:"topics" ssz-max:"4"`
	// Supplied by the contract, usually ABI-encoded
	Data hexutil.Bytes `json:"data" ssz-max:"6000"`
}

type DebugLog struct {
	// Message contains the log message
	Message []byte `json:"message" ssz-max:"6000"`
	// Data contains array of integers
	Data []Uint256 `json:"data" ssz-max:"6000"`
}

func NewLog(address Address, data []byte, topics []common.Hash) *Log {
	return &Log{
		Address: address,
		Topics:  topics,
		Data:    data,
	}
}

func (l *Log) TopicsNum() int {
	return len(l.Topics)
}
