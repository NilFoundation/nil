package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func calculateStateChange(
	newEs, oldEs *execution.ExecutionState, prevOverrides *rpctypes.StateOverrides,
) (rpctypes.StateOverrides, error) {
	stateOverrides := make(rpctypes.StateOverrides)

	for addr, as := range newEs.Accounts {
		var contract rpctypes.Contract
		var hasUpdates bool
		oldAs, err := oldEs.GetAccount(addr)
		if err != nil {
			return nil, err
		}

		if oldAs == nil {
			hasUpdates = true
			contract.Seqno = common.Ptr(as.GetSeqno())
			contract.ExtSeqno = common.Ptr(as.GetExtSeqno())
			contract.Balance = common.Ptr(as.GetBalance())
			contract.Code = (*hexutil.Bytes)(common.Ptr(as.GetCode()))
			contract.State = (*map[common.Hash]common.Hash)(common.Ptr(as.GetFullState()))
			contract.AsyncContext = common.Ptr(as.GetAllAsyncContexts())
		} else {
			if as.GetSeqno() != oldAs.GetSeqno() {
				hasUpdates = true
				contract.Seqno = common.Ptr(as.GetSeqno())
			}

			if as.GetExtSeqno() != oldAs.GetExtSeqno() {
				hasUpdates = true
				contract.ExtSeqno = common.Ptr(as.GetExtSeqno())
			}

			if !as.GetBalance().Eq(oldAs.GetBalance()) {
				hasUpdates = true
				contract.Balance = common.Ptr(as.GetBalance())
			}

			if !bytes.Equal(as.GetCode(), oldAs.GetCode()) {
				hasUpdates = true
				contract.Code = (*hexutil.Bytes)(common.Ptr(as.GetCode()))
			}

			for key, value := range as.GetFullState() {
				oldVal, err := oldAs.GetState(key)
				if err != nil {
					return nil, err
				}
				if value != oldVal {
					hasUpdates = true
					if contract.StateDiff == nil {
						m := make(map[common.Hash]common.Hash)
						contract.StateDiff = &m
					}
					(*contract.StateDiff)[key] = value
				}
			}

			asyncContextHasUpdates := len(as.GetAllAsyncContexts()) != len(oldAs.GetAllAsyncContexts())
			if !asyncContextHasUpdates {
				for key, value := range as.GetAllAsyncContexts() {
					oldVal, err := oldAs.GetAsyncContext(key)
					if err != nil {
						if !errors.Is(err, db.ErrKeyNotFound) {
							return nil, err
						}
						asyncContextHasUpdates = true
						break
					}
					if value.ResponseProcessingGas != oldVal.ResponseProcessingGas {
						asyncContextHasUpdates = true
						break
					}
				}
			}

			if asyncContextHasUpdates {
				hasUpdates = true
				contract.AsyncContext = common.Ptr(as.GetAllAsyncContexts())
			}
		}

		if hasUpdates {
			stateOverrides[addr] = contract
		}
	}

	if prevOverrides != nil {
		for addr, override := range *prevOverrides {
			if addr.ShardId() == newEs.ShardId {
				continue
			}
			stateOverrides[addr] = override
		}
	}

	return stateOverrides, nil
}

func (api *localShardApiRo) handleOutTransactions(
	ctx context.Context,
	outTxns []*types.OutboundTransaction,
	mainBlockHash common.Hash,
	childBlocks []common.Hash,
	overrides *rpctypes.StateOverrides,
) ([]*rpctypes.OutTransaction, error) {
	outTransactions := make([]*rpctypes.OutTransaction, len(outTxns))

	for i, outTxn := range outTxns {
		raw, err := outTxn.MarshalNil()
		if err != nil {
			return nil, err
		}

		args := rpctypes.CallArgs{
			Transaction: (*hexutil.Bytes)(&raw),
		}

		res, err := api.nodeApi.Call(
			ctx,
			args,
			rawapitypes.BlockHashWithChildrenAsBlockReferenceOrHashWithChildren(mainBlockHash, childBlocks),
			overrides)
		if err != nil {
			return nil, err
		}

		outTransactions[i] = &rpctypes.OutTransaction{
			TransactionBytes: raw,
			ForwardKind:      outTxn.ForwardKind,
			Data:             res.Data,
			CoinsUsed:        res.CoinsUsed,
			OutTransactions:  res.OutTransactions,
			BaseFee:          res.BaseFee,
			Error:            res.Error,
			Logs:             res.Logs,
		}

		if overrides != nil {
			for k, v := range res.StateOverrides {
				(*overrides)[k] = v
			}
		}
	}

	return outTransactions, nil
}

func (api *localShardApiRo) Call(
	ctx context.Context, args rpctypes.CallArgs,
	mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren,
	overrides *rpctypes.StateOverrides,
) (*rpctypes.CallResWithGasPrice, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	txn, err := args.ToTransaction()
	if err != nil {
		return nil, err
	}

	shardId := txn.To.ShardId()
	if shardId != api.shardId() {
		return nil, fmt.Errorf("destination shard %d is not equal to the instance shard %d", shardId, api.shard)
	}

	var mainBlockHash common.Hash
	var childBlocks []common.Hash
	if mainBlockReferenceOrHashWithChildren.IsReference() {
		mainBlockData, err := api.nodeApi.GetFullBlockData(
			ctx,
			types.MainShardId,
			mainBlockReferenceOrHashWithChildren.Reference())
		if err != nil {
			return nil, err
		}
		mainBlock, err := mainBlockData.DecodeBytes()
		if err != nil {
			return nil, err
		}
		mainBlockHash = mainBlock.Hash(types.MainShardId)
		childBlocks = mainBlockData.ChildBlocks
	} else {
		mainBlockHash, childBlocks = mainBlockReferenceOrHashWithChildren.HashAndChildren()
	}

	var hash common.Hash
	if !shardId.IsMainShard() {
		if len(childBlocks) < int(shardId) {
			return nil, fmt.Errorf("%w: main shard includes only %d blocks",
				makeShardNotFoundError(methodNameChecked("Call"), shardId), len(childBlocks))
		}
		hash = childBlocks[shardId-1]
	} else {
		hash = mainBlockHash
	}

	block, err := db.ReadBlock(tx, shardId, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to read block %s: %w", hash, err)
	}

	configAccessor, err := config.NewConfigAccessorFromBlockWithTx(tx, block, shardId)
	if err != nil {
		return nil, fmt.Errorf("failed to create config accessor: %w", err)
	}

	es, err := execution.NewExecutionState(tx, shardId, execution.StateParams{
		Block:          block,
		ConfigAccessor: configAccessor,
		StateAccessor:  api.accessor,
		Mode:           execution.ModeReadOnly,
	})
	if err != nil {
		return nil, err
	}
	es.MainShardHash = mainBlockHash

	if overrides != nil {
		if err := overrides.Override(es); err != nil {
			return nil, err
		}
	}

	if txn.IsDeploy() {
		if err := execution.ValidateDeployTransaction(txn); err != nil {
			return nil, err
		}
	}

	var payer execution.Payer
	switch {
	case args.Transaction == nil:
		// "args.Transaction == nil" mean that it's a root transaction
		// and we don't want to withdraw any payment for it.
		// Because it's quite useful for read-only methods.
		payer = execution.NewDummyPayer()
	case txn.IsInternal():
		payer = execution.NewTransactionPayer(txn, es)
	default:
		var toAs execution.AccountState
		if toAs, err = es.GetAccount(txn.To); err != nil {
			return nil, err
		} else if toAs == nil {
			return nil, rpctypes.ErrToAccNotFound
		}
		payer = execution.NewAccountPayer(toAs, txn)
	}

	txn.TxId = es.InTxCounts[txn.From.ShardId()]
	txnHash := es.AddInTransaction(txn)
	res := es.HandleTransaction(ctx, txn, payer)

	result := &rpctypes.CallResWithGasPrice{
		Data:      res.ReturnData,
		CoinsUsed: res.CoinsUsed(),
		Logs:      es.Logs[txnHash],
		DebugLogs: es.DebugLogs[txnHash],
	}

	if res.Failed() {
		result.Error = res.GetError().Error()
		return result, nil
	}

	esOld, err := execution.NewExecutionState(tx, shardId, execution.StateParams{
		Block:          block,
		ConfigAccessor: config.GetStubAccessor(),
		StateAccessor:  api.accessor,
		Mode:           execution.ModeReadOnly,
	})
	if err != nil {
		return nil, err
	}
	stateOverrides, err := calculateStateChange(es, esOld, overrides)
	if err != nil {
		return nil, err
	}

	execOutTransactions := es.OutTransactions[txnHash]
	outTransactions, err := api.handleOutTransactions(
		ctx,
		execOutTransactions,
		mainBlockHash,
		childBlocks,
		&stateOverrides,
	)
	if err != nil {
		return nil, err
	}

	result.OutTransactions = outTransactions
	result.StateOverrides = stateOverrides
	result.BaseFee = es.BaseFee
	return result, nil
}
