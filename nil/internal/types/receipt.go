package types

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/ethereum/go-ethereum/rlp"
)

type Receipts []*Receipt

const (
	ReceiptMaxLogsSize      = 1000
	ReceiptMaxDebugLogsSize = 1000
)

type Receipt struct {
	Success     bool        `json:"success"`
	Status      ErrorCode   `json:"status"`
	GasUsed     Gas         `json:"gasUsed"`
	Forwarded   Value       `json:"forwarded"`
	Logs        []*Log      `json:"logs"`
	DebugLogs   []*DebugLog `json:"debugLogs"`
	OutTxnIndex uint32      `json:"outTxnIndex"`
	OutTxnNum   uint32      `json:"outTxnNum"`
	FailedPc    uint32      `json:"failedPc"`

	TxnHash         common.Hash `json:"transactionHash"`
	ContractAddress Address     `json:"contractAddress"`
}

func (r *Receipt) Hash() common.Hash {
	return ToShardedHash(common.MustKeccak(r), ShardIdFromHash(r.TxnHash))
}

func (r *Receipt) UnmarshalNil(buf []byte) error {
	return rlp.DecodeBytes(buf, r)
}

func (r Receipt) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(&r)
}
