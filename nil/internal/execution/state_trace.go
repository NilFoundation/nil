package execution

import (
	"errors"
	"fmt"
	"os"
	"sync"

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

func (bt *BlocksTracer) PrintMessage(msg *types.Message) {
	bt.Printf("hash: %s\n", msg.Hash().Hex())
	bt.Printf("flags: %v\n", msg.Flags)
	bt.Printf("seqno: %d\n", msg.Seqno)
	bt.Printf("from: %s\n", msg.From.Hex())
	bt.Printf("to: %s\n", msg.To.Hex())
	bt.Printf("refundTo: %s\n", msg.RefundTo.Hex())
	bt.Printf("bounceTo: %s\n", msg.BounceTo.Hex())
	bt.Printf("value: %s\n", msg.Value)
	bt.Printf("fee: %s\n", msg.FeeCredit)
	if msg.IsRequestOrResponse() {
		bt.Printf("requestId: %d\n", msg.RequestId)
	}
	if len(msg.RequestChain) > 0 {
		bt.Printf("requestChain: [")
		for i, req := range msg.RequestChain {
			if i > 0 {
				fmt.Fprintf(bt.file, ", %d", req.Id)
			} else {
				fmt.Fprintf(bt.file, "%d", req.Id)
			}
		}
		fmt.Fprintln(bt.file, "]")
	}
	if len(msg.Data) < 1024 {
		bt.Printf("data: %s\n", hexutil.Encode(msg.Data))
	} else {
		bt.Printf("data_size: %d\n", len(msg.Data))
	}
	if len(msg.Currency) > 0 {
		bt.Printf("currency:\n")
		for _, curr := range msg.Currency {
			bt.WithIndent(func(t *BlocksTracer) {
				bt.Printf("%s:%s\n", hexutil.Encode(curr.Currency[:]), curr.Balance.String())
			})
		}
	}
}

func (bt *BlocksTracer) Trace(es *ExecutionState, block *types.Block) {
	bt.lock.Lock()
	defer bt.lock.Unlock()

	root := mpt.NewDbReader(es.tx, es.ShardId, db.ContractTrieTable)
	root.SetRootHash(block.SmartContractsRoot)
	contractsNum := 0
	for range root.Iterate() {
		contractsNum++
	}

	if !printEmptyBlocks && len(es.InMessages) == 0 {
		return
	}

	bt.Printf("-\n")
	bt.WithIndent(func(t *BlocksTracer) {
		bt.Printf("shard: %d\n", es.ShardId)
		bt.Printf("id: %d\n", block.Id)
		bt.Printf("hash: %s\n", block.Hash(es.ShardId).Hex())
		bt.Printf("gas_price: %v\n", es.GasPrice)
		bt.Printf("contracts_num: %d\n", contractsNum)
		if len(es.InMessages) != 0 {
			bt.Printf("in_messages:\n")
			for i, msg := range es.InMessages {
				bt.WithIndent(func(t *BlocksTracer) {
					bt.Printf("%d:\n", i)

					bt.WithIndent(func(t *BlocksTracer) {
						bt.PrintMessage(msg)
						bt.Printf("receipt:\n")
						receipt := es.Receipts[i]

						bt.WithIndent(func(t *BlocksTracer) {
							bt.Printf("success: %t\n", receipt.Success)
							if !receipt.Success {
								bt.Printf("status: %s\n", receipt.Status.String())
								bt.Printf("pc: %d\n", receipt.FailedPc)
							}
							bt.Printf("gas_used: %d\n", receipt.GasUsed)
							bt.Printf("msg_hash: %s\n", receipt.MsgHash.Hex())
							bt.Printf("address: %s\n", receipt.ContractAddress.Hex())
						})

						outMessages, ok := es.OutMessages[msg.Hash()]
						if ok {
							bt.Printf("out_messages:\n")

							bt.WithIndent(func(t *BlocksTracer) {
								for j, outMsg := range outMessages {
									bt.Printf("%d:\n", j)
									bt.WithIndent(func(t *BlocksTracer) {
										bt.PrintMessage(outMsg.Message)
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

func (bt *BlocksTracer) Printf(format string, args ...interface{}) {
	if _, err := bt.file.WriteString(bt.indent); err != nil {
		panic(err)
	}
	if _, err := fmt.Fprintf(bt.file, format, args...); err != nil {
		panic(err)
	}
}
