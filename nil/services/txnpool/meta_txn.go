package txnpool

import (
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type metaTxn struct {
	*types.TxnWithHash
	effectivePriorityFee types.Value
	bestIndex            int
	discardReason        DiscardReason
}

func newMetaTxn(txn *types.Transaction, baseFee types.Value) *metaTxn {
	effectivePriorityFee, valid := execution.GetEffectivePriorityFee(baseFee, txn)
	var discardReason DiscardReason
	if !valid {
		discardReason = TooSmallMaxFee
	}

	return &metaTxn{
		TxnWithHash:          types.NewTxnWithHash(txn),
		effectivePriorityFee: effectivePriorityFee,
		discardReason:        discardReason,
		bestIndex:            -1,
	}
}

func (m *metaTxn) Clone() *metaTxn {
	return &metaTxn{
		TxnWithHash:          m.TxnWithHash,
		effectivePriorityFee: m.effectivePriorityFee,
		bestIndex:            m.bestIndex,
		discardReason:        m.discardReason,
	}
}

func (m *metaTxn) IsValid() bool {
	return m.discardReason == NotSet
}

func (m *metaTxn) GetDiscardReason() DiscardReason {
	return m.discardReason
}

func (m *metaTxn) IsInQueue() bool {
	return m.bestIndex >= 0
}
