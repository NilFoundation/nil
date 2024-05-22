package types

import (
	"github.com/NilFoundation/nil/common"
)

type Receipts []*Receipt

type Receipt struct {
	Success bool   `json:"success"`
	GasUsed uint32 `json:"gasUsed"`
	Bloom   Bloom  `json:"bloom"`
	Logs    []*Log `json:"logs"    ssz-max:"1000"`

	MsgHash         common.Hash    `json:"messageHash"`
	ContractAddress common.Address `json:"contractAddress"`

	BlockHash   common.Hash `json:"blockHash,omitempty"`
	BlockNumber BlockNumber `json:"blockNumber,omitempty"`
	MsgIndex    uint64      `json:"messageIndex"`
}

func (r *Receipt) AddLog(log *Log) {
	r.Logs = append(r.Logs, log)
}

func (r *Receipt) LogsNum() int {
	return len(r.Logs)
}
