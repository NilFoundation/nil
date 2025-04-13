package execution

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
)

const (
	printToStdout    = true
	printEmptyBlocks = false
)

type BlocksTracer struct {
	file   *os.File
	lock   *sync.Mutex
	indent string
}

func NewBlocksTracer() (*BlocksTracer, error) {
	var err error
	bt := &BlocksTracer{
		lock:   &sync.Mutex{},
		indent: "",
	}
	if printToStdout {
		bt.file = os.Stdout
	} else {
		bt.file, err = os.OpenFile("blocks.txt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o777)
		if err != nil || bt.file == nil {
			return nil, errors.New("can not open trace blocks file")
		}
	}
	return bt, nil
}

func (bt *BlocksTracer) Close() error {
	return bt.file.Close()
}

func (bt *BlocksTracer) PrintTransaction(txn *types.Transaction, hash common.Hash) {
	bt.Printf("hash: %s\n", hash.Hex())
	bt.Printf("flags: %v\n", txn.Flags)
	bt.Printf("seqno: %d\n", txn.Seqno)
	bt.Printf("from: %s\n", txn.From.Hex())
	bt.Printf("to: %s\n", txn.To.Hex())
	bt.Printf("refundTo: %s\n", txn.RefundTo.Hex())
	bt.Printf("bounceTo: %s\n", txn.BounceTo.Hex())
	bt.Printf("value: %s\n", txn.Value)
	bt.Printf("fee: %s\n", txn.FeeCredit)
	if txn.IsRequestOrResponse() {
		bt.Printf("requestId: %d\n", txn.RequestId)
	}
	if len(txn.RequestChain) > 0 {
		bt.Printf("requestChain: [")
		for i, req := range txn.RequestChain {
			if i > 0 {
				fmt.Fprintf(bt.file, ", %d", req.Id)
			} else {
				fmt.Fprintf(bt.file, "%d", req.Id)
			}
		}
		fmt.Fprintln(bt.file, "]")
	}
	if len(txn.Data) < 1024 {
		bt.Printf("data: %s\n", hexutil.Encode(txn.Data))
	} else {
		bt.Printf("data_size: %d\n", len(txn.Data))
	}
	if len(txn.Token) > 0 {
		bt.Printf("token:\n")
		for _, tok := range txn.Token {
			bt.WithIndent(func(t *BlocksTracer) {
				bt.Printf("%s:%s\n", hexutil.Encode(tok.Token[:]), tok.Balance.String())
			})
		}
	}
}

func (bt *BlocksTracer) Trace(es *ExecutionState, block *types.Block, blockHash common.Hash) {
	bt.lock.Lock()
	defer bt.lock.Unlock()

	root := mpt.NewDbReader(es.tx, es.ShardId, db.ContractTrieTable)
	root.SetRootHash(block.SmartContractsRoot)
	contractsNum := 0
	for range root.Iterate() {
		contractsNum++
	}

	if !printEmptyBlocks && len(es.InTransactions) == 0 {
		return
	}

	bt.Printf("-\n")
	bt.WithIndent(func(t *BlocksTracer) {
		bt.Printf("shard: %d\n", es.ShardId)
		bt.Printf("id: %d\n", block.Id)
		bt.Printf("hash: %s\n", blockHash.Hex())
		bt.Printf("gas_price: %v\n", es.GasPrice)
		bt.Printf("contracts_num: %d\n", contractsNum)
		if len(es.InTransactions) != 0 {
			bt.Printf("in_transactions:\n")
			for i, txn := range es.InTransactions {
				bt.WithIndent(func(t *BlocksTracer) {
					bt.Printf("%d:\n", i)

					bt.WithIndent(func(t *BlocksTracer) {
						txnHash := es.InTransactionHashes[i]
						bt.PrintTransaction(txn, txnHash)
						bt.Printf("receipt:\n")
						receipt := es.Receipts[i]

						bt.WithIndent(func(t *BlocksTracer) {
							bt.Printf("success: %t\n", receipt.Success)
							if !receipt.Success {
								bt.Printf("status: %s\n", receipt.Status.String())
								bt.Printf("pc: %d\n", receipt.FailedPc)
							}
							bt.Printf("gas_used: %d\n", receipt.GasUsed)
							bt.Printf("txn_hash: %s\n", receipt.TxnHash.Hex())
							bt.Printf("address: %s\n", receipt.ContractAddress.Hex())
						})

						outTransactions, ok := es.OutTransactions[txnHash]
						if ok {
							bt.Printf("out_transactions:\n")

							bt.WithIndent(func(t *BlocksTracer) {
								for j, outTxn := range outTransactions {
									bt.Printf("%d:\n", j)
									bt.WithIndent(func(t *BlocksTracer) {
										bt.PrintTransaction(outTxn.Transaction, outTxn.TxnHash)
									})
								}
							})
						}
					})
				})
			}
		}
	})

	if len(bt.indent) != 0 {
		panic("Trace method is invalid")
	}
}

func (bt *BlocksTracer) WithIndent(f func(*BlocksTracer)) {
	bt.indent += "  "
	f(bt)
	bt.indent = bt.indent[2:]
}

func (bt *BlocksTracer) Printf(format string, args ...any) {
	if _, err := bt.file.WriteString(bt.indent); err != nil {
		panic(err)
	}
	if _, err := fmt.Fprintf(bt.file, format, args...); err != nil {
		panic(err)
	}
}
