package types

import (
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	LogMaxDataSize         = 6000
	DebugLogMaxMessageSize = 6000
	DebugLogMaxDataSize    = 6000
)

type Logs []*Log

type Log struct {
	// Address of the contract that generated the event
	Address Address `json:"address"`
	// List of topics provided by the contract
	Topics []common.Hash `json:"topics"`
	// Supplied by the contract, usually ABI-encoded
	Data hexutil.Bytes `json:"data"`
}

func (l Log) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(&l)
}

type DebugLog struct {
	// Message contains the log message
	Message []byte `json:"message"`
	// Data contains array of integers
	Data []Uint256 `json:"data"`
}

func NewLog(address Address, data []byte, topics []common.Hash) (*Log, error) {
	if len(data) > LogMaxDataSize {
		return nil, errors.New("log size is too long")
	}

	return &Log{
		Address: address,
		Topics:  topics,
		Data:    data,
	}, nil
}

func NewDebugLog(message []byte, data []Uint256) (*DebugLog, error) {
	if len(message) > DebugLogMaxMessageSize {
		return nil, errors.New("debug log message size is too long")
	}
	if len(data) > DebugLogMaxDataSize {
		return nil, errors.New("debug log data size is too long")
	}

	return &DebugLog{
		Message: message,
		Data:    data,
	}, nil
}

func (l *Log) TopicsNum() int {
	return len(l.Topics)
}
