package jsonrpc

import (
	"context"
	"math/big"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
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

	es, err := execution.NewROExecutionState(tx, shardId, hash, timer, 1)
	if err != nil {
		return nil, err
	}

	blockContext, err := execution.NewEVMBlockContext(es)
	if err != nil {
		return nil, err
	}

	gasPrice, err := api.GasPrice(ctx, args.To.ShardId())
	if err != nil {
		return nil, err
	}
	gas := args.FeeCredit.ToGas(types.NewValueFromBigMust((*big.Int)(gasPrice)))

	evm := vm.NewEVM(blockContext, es, args.From)
	ret, _, err := evm.Call((vm.AccountRef)(args.From), args.To, args.Data, gas.Uint64(), args.Value.Int())
	return ret, err
}
