package execution

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
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

func (bt *BlocksTracer) Trace(es *ExecutionState, block *types.Block) {
	bt.lock.Lock()
	defer bt.lock.Unlock()

	root := mpt.NewReaderWithRoot(es.tx, es.ShardId, db.ContractTrieTable, block.SmartContractsRoot)
	contractsNum := 0
	for range root.Iterate() {
		contractsNum++
	}

	if !printEmptyBlocks && len(es.InMessages) == 0 {
		return
	}
	printMessage := func(msg *types.Message) {
		bt.printf("hash: %s\n", msg.Hash().Hex())
		bt.printf("flags: %v\n", msg.Flags)
		bt.printf("seqno: %d\n", msg.Seqno)
		bt.printf("from: %s\n", msg.From.Hex())
		bt.printf("to: %s\n", msg.To.Hex())
		bt.printf("refundTo: %s\n", msg.RefundTo.Hex())
		bt.printf("bounceTo: %s\n", msg.BounceTo.Hex())
		bt.printf("value: %s\n", msg.Value.String())
		bt.printf("fee: %s\n", msg.FeeCredit.String())
		if len(msg.Data) < 1024 {
			bt.printf("data: %s\n", hexutil.Encode(msg.Data))
		} else {
			bt.printf("data_size: %d\n", len(msg.Data))
		}
		if len(msg.Currency) > 0 {
			bt.printf("currency:\n")
			for _, curr := range msg.Currency {
				bt.withIndent(func(t *BlocksTracer) {
					bt.printf("%s:%s\n", hexutil.Encode(curr.Currency[:]), curr.Balance.String())
				})
			}
		}
	}

	bt.printf("-\n")
	bt.withIndent(func(t *BlocksTracer) {
		bt.printf("shard: %d\n", es.ShardId)
		bt.printf("id: %d\n", block.Id)
		bt.printf("hash: %s\n", block.Hash().Hex())
		bt.printf("contracts_num: %d\n", contractsNum)
		if len(es.InMessages) != 0 {
			bt.printf("in_messages:\n")
			for i, msg := range es.InMessages {
				bt.withIndent(func(t *BlocksTracer) {
					bt.printf("%d:\n", i)

					bt.withIndent(func(t *BlocksTracer) {
						printMessage(msg)
						bt.printf("receipt:\n")
						receipt := es.Receipts[i]

						bt.withIndent(func(t *BlocksTracer) {
							bt.printf("success: %t\n", receipt.Success)
							bt.printf("status: %s\n", receipt.Status.String())
							bt.printf("gas_used: %d\n", receipt.GasUsed)
							bt.printf("msg_hash: %s\n", receipt.MsgHash.Hex())
							bt.printf("address: %s\n", receipt.ContractAddress.Hex())
						})

						outMessages, ok := es.OutMessages[msg.Hash()]
						if ok {
							bt.printf("out_messages:\n")

							bt.withIndent(func(t *BlocksTracer) {
								for j, outMsg := range outMessages {
									bt.printf("%d:\n", j)
									bt.withIndent(func(t *BlocksTracer) {
										printMessage(outMsg)
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

func (bt *BlocksTracer) withIndent(f func(*BlocksTracer)) {
	bt.indent += "  "
	f(bt)
	bt.indent = bt.indent[2:]
}

func (bt *BlocksTracer) printf(format string, args ...interface{}) {
	if _, err := bt.file.WriteString(bt.indent); err != nil {
		panic(err)
	}
	if _, err := fmt.Fprintf(bt.file, format, args...); err != nil {
		panic(err)
	}
}
