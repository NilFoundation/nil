package types

import (
	"github.com/NilFoundation/nil/nil/common"
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

func (r *Receipt) Hash() common.Hash {
	return common.MustPoseidonSSZ(r)
}
