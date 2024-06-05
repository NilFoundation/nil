package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/rpc/transport"
)

// Call implements eth_call. Executes a new message call immediately without creating a transaction on the block chain.
func (api *APIImpl) Call(ctx context.Context, args CallArgs, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Bytes, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	timer := common.NewTimer()
	shardId := args.From.ShardId()

	hash, err := api.extractBlockHash(tx, shardId, blockNrOrHash)
	if err != nil {
		return nil, err
	}

	es, err := execution.NewROExecutionState(tx, shardId, hash, timer)
	if err != nil {
		return nil, err
	}

	blockContext := execution.NewEVMBlockContext(es)

	evm := vm.NewEVM(blockContext, es)

	gas := args.GasLimit.Uint64()
	ret, _, err := evm.Call((vm.AccountRef)(args.From), args.To, args.Data, gas, &args.Value.Int)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
