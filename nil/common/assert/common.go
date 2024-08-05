package assert

type TxFinishCb func()

type TxLedger interface {
	TxOnStart(stack []byte) TxFinishCb

	CheckLeakyTransactions()
}
