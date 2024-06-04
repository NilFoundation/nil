package types

import (
	"github.com/NilFoundation/nil/common"
)

type Receipts []*Receipt

type Receipt struct {
	Success     bool   `json:"success"`
	GasUsed     uint32 `json:"gasUsed"`
	Bloom       Bloom  `json:"bloom"`
	Logs        []*Log `json:"logs" ssz-max:"1000"`
	OutMsgIndex uint32 `json:"outMsgIndex"`

	MsgHash         common.Hash `json:"messageHash"`
	ContractAddress Address     `json:"contractAddress"`

	MsgIndex uint64 `json:"messageIndex"`
}

func (r *Receipt) AddLog(log *Log) {
	r.Logs = append(r.Logs, log)
}

func (r *Receipt) LogsNum() int {
	return len(r.Logs)
}

func (r *Receipt) Hash() common.Hash {
	h, err := common.PoseidonSSZ(r)
	common.FatalIf(err, nil, "Can't get receipt hash")
	return h
}
