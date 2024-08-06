package types

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
)

type Receipts []*Receipt

type Receipt struct {
	Success     bool          `json:"success"`
	Status      MessageStatus `json:"status"`
	GasUsed     Gas           `json:"gasUsed"`
	Forwarded   Value         `json:"forwarded"`
	Bloom       Bloom         `json:"bloom"`
	Logs        []*Log        `json:"logs" ssz-max:"1000"`
	OutMsgIndex uint32        `json:"outMsgIndex"`
	OutMsgNum   uint32        `json:"outMsgNum"`

	MsgHash         common.Hash `json:"messageHash"`
	ContractAddress Address     `json:"contractAddress"`
}

func (r *Receipt) AddLog(log *Log) {
	r.Logs = append(r.Logs, log)
}

func (r *Receipt) LogsNum() int {
	return len(r.Logs)
}

func (r *Receipt) Hash() common.Hash {
	h, err := common.PoseidonSSZ(r)
	check.PanicIfErr(err)
	return h
}
