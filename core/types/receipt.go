package types

import (
	"github.com/NilFoundation/nil/common"
)

type Receipts []*Receipt

type Receipt struct {
	Success bool   `json:"success"`
	GasUsed uint32 `json:"gasUsed"`
	Bloom   Bloom  `json:"bloom"`
	Logs    []*Log `json:"logs" ssz-max:"1000"`

	TxHash          common.Hash    `json:"transactionHash"`
	ContractAddress common.Address `json:"contractAddress"`

	BlockHash   common.Hash `json:"blockHash,omitempty"`
	BlockNumber uint64      `json:"blockNumber,omitempty"`
	TxIndex     uint64      `json:"transactionIndex"`
}

func (r *Receipt) AddLog(log *Log) {
	r.Logs = append(r.Logs, log)
}
