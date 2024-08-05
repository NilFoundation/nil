//go:build assert

package assert

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/common/concurrent"
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
	var leakyTxStak []byte
	l.runningTxs.Range(func(k uint64, v []byte) error {
		leakyTxStak = v
		// return error to exit from range loop
		return errors.New("")
	})

	if len(leakyTxStak) > 0 {
		panic(fmt.Sprintf("Transaction wasn't terminated:\n%s", leakyTxStak))
	}
}

func NewTxLedger() TxLedger {
	return &txLedger{runningTxs: concurrent.NewMap[uint64, []byte]()}
}
