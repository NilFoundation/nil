package types

import (
	"github.com/NilFoundation/nil/nil/common"
)

type Receipts []*Receipt

type Receipt struct {
	Success     bool        `json:"success"`
	Status      ErrorCode   `json:"status"`
	GasUsed     Gas         `json:"gasUsed"`
	Forwarded   Value       `json:"forwarded"`
	Bloom       Bloom       `json:"bloom"`
	Logs        []*Log      `json:"logs" ssz-max:"1000"`
	DebugLogs   []*DebugLog `json:"debugLogs" ssz-max:"1000"`
	OutTxnIndex uint32      `json:"outTxnIndex"`
	OutTxnNum   uint32      `json:"outTxnNum"`
	FailedPc    uint32      `json:"failedPc"`

	TxnHash         common.Hash `json:"transactionHash"`
	ContractAddress Address     `json:"contractAddress"`
}

func (r *Receipt) Hash() common.Hash {
	return ToShardedHash(common.MustPoseidonSSZ(r), ShardIdFromHash(r.TxnHash))
}

//go:generate go run github.com/NilFoundation/fastssz/sszgen --path receipt.go -include ../../common/hexutil/bytes.go,../../common/length.go,address.go,gas.go,value.go,block.go,bloom.go,log.go,transaction.go,exec_errors.go,../../common/hash.go,uint256.go --objs Receipt
