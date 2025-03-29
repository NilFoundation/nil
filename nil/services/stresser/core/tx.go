package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

var ErrTxFailed = errors.New("transaction is failed")

const defaultTimeout = 15 * time.Second

type Transaction struct {
	Hash         common.Hash
	Receipt      *jsonrpc.RPCReceipt
	Error        error
	StartTm      time.Time
	ExpectedFail bool
	Timeout      time.Duration
}

func NewTransaction(hash common.Hash) *Transaction {
	return &Transaction{Hash: hash, StartTm: time.Now(), Timeout: defaultTimeout}
}

func (t *Transaction) CheckFinished(ctx context.Context, c *Helper) bool {
	t.Receipt, t.Error = c.Client.GetInTransactionReceipt(ctx, t.Hash)
	if complete := t.Error == nil && t.Receipt.IsComplete(); !complete {
		if time.Since(t.StartTm) > t.Timeout {
			t.Error = fmt.Errorf("transaction timed out(timeout=%fs)", t.Timeout.Seconds())
			return true
		}
		return false
	}
	if !t.Receipt.AllSuccess() {
		t.Error = ErrTxFailed
	} else if t.ExpectedFail {
		t.Error = errors.New("transaction is expected to fail")
	}
	return true
}

func (t *Transaction) Dump(full bool) string {
	res := fmt.Sprintf("Tx %s\n", t.Hash.Hex())
	txRes := "success"
	if t.Error != nil {
		txRes = t.Error.Error()
	}
	res += "  Result: " + txRes + "\n"
	if full {
		if t.Receipt != nil {
			d, err := json.MarshalIndent(t.Receipt, "", "  ")
			check.PanicIfErr(err)
			res += string(d)
		}
	}
	return res
}
