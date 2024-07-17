//go:build assert

package assert

import (
	"fmt"
	"sync/atomic"

	"github.com/NilFoundation/nil/common/concurrent"
)

const Enable = true

type txLedger struct {
	runningTxs *concurrent.Map[uint64, []byte]
	txId       atomic.Uint64
}

func (l *txLedger) TxOnStart(stack []byte) TxFinishCb {
	uid := l.txId.Add(1)
	l.runningTxs.Put(uid, stack)

	return func() {
		l.runningTxs.Delete(uid)
	}
}

func (l *txLedger) CheckLeakyTransactions() {
	for kv := range l.runningTxs.Iter() {
		panic(fmt.Sprintf("Transaction wasn't terminated:\n%s", kv.Value))
	}
}

func NewTxLedger() TxLedger {
	return &txLedger{runningTxs: concurrent.NewMap[uint64, []byte]()}
}
