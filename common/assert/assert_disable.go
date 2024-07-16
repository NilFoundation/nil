//go:build !assert

package assert

const Enable = false

type txLedger struct{}

func (*txLedger) TxOnStart([]byte) TxFinishCb {
	return func() {}
}

func (*txLedger) CheckLeakyTransactions() {}

func NewTxLedger() TxLedger {
	return new(txLedger)
}
