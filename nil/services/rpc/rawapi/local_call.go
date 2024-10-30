package rawapi

import (
	"bytes"
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

func calculateStateChange(newEs, oldEs *execution.ExecutionState) (rpctypes.StateOverrides, error) {
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
			contract.Seqno = &as.Seqno
			contract.ExtSeqno = &as.ExtSeqno
			contract.Balance = &as.Balance
			contract.Code = (*hexutil.Bytes)(&as.Code)
			contract.State = (*map[common.Hash]common.Hash)(&as.State)
		} else {
			if as.Seqno != oldAs.Seqno {
				hasUpdates = true
				contract.Seqno = &as.Seqno
			}

			if as.ExtSeqno != oldAs.ExtSeqno {
				hasUpdates = true
				contract.ExtSeqno = &as.ExtSeqno
			}

			if !as.Balance.Eq(oldAs.Balance) {
				hasUpdates = true
				contract.Balance = &as.Balance
			}

			if !bytes.Equal(as.Code, oldAs.Code) {
				hasUpdates = true
				contract.Code = (*hexutil.Bytes)(&as.Code)
			}

			for key, value := range as.State {
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
		}

		if hasUpdates {
			stateOverrides[addr] = contract
		}
	}
	return stateOverrides, nil
}

func (api *LocalShardApi) handleOutMessages(
	ctx context.Context,
	outMsgs []*types.OutboundMessage,
	mainBlockHash common.Hash,
	childBlocks []common.Hash,
	overrides *rpctypes.StateOverrides,
) ([]*rpctypes.OutMessage, error) {
	outMessages := make([]*rpctypes.OutMessage, len(outMsgs))

	for i, outMsg := range outMsgs {
		raw, err := outMsg.Message.MarshalSSZ()
		if err != nil {
			return nil, err
		}

		args := rpctypes.CallArgs{
			Message: (*hexutil.Bytes)(&raw),
		}

		res, err := api.nodeApi.Call(
			ctx,
			args,
			rawapitypes.BlockHashWithChildrenAsBlockReferenceOrHashWithChildren(mainBlockHash, childBlocks),
			overrides,
			true)
		if err != nil {
			return nil, err
		}

		outMessages[i] = &rpctypes.OutMessage{
			MessageSSZ:  raw,
			ForwardKind: outMsg.ForwardKind,
			Data:        res.Data,
			CoinsUsed:   res.CoinsUsed,
			OutMessages: res.OutMessages,
			GasPrice:    res.GasPrice,
			Error:       res.Error,
		}

		if overrides != nil {
			for k, v := range res.StateOverrides {
				(*overrides)[k] = v
			}
		}
	}

	return outMessages, nil
}

func (api *LocalShardApi) Call(
	ctx context.Context, args rpctypes.CallArgs, mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren, overrides *rpctypes.StateOverrides, emptyMessageIsRoot bool,
) (*rpctypes.CallResWithGasPrice, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	msg, err := args.ToMessage()
	if err != nil {
		return nil, err
	}

	timer := common.NewTimer()
	shardId := msg.To.ShardId()
	if shardId != api.ShardId {
		return nil, fmt.Errorf("destination shard %d is not equal to the instance shard %d", shardId, api.ShardId)
	}

	var mainBlockHash common.Hash
	var childBlocks []common.Hash
	if mainBlockReferenceOrHashWithChildren.IsReference() {
		mainBlockData, err := api.nodeApi.GetFullBlockData(ctx, types.MainShardId, mainBlockReferenceOrHashWithChildren.Reference())
		if err != nil {
			return nil, err
		}
		mainBlock, err := mainBlockData.DecodeSSZ()
		if err != nil {
			return nil, err
		}
		mainBlockHash = mainBlock.Hash()
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

	es, err := execution.NewROExecutionState(tx, shardId, hash, timer, 1)
	if err != nil {
		return nil, err
	}

	if overrides != nil {
		if err := overrides.Override(es); err != nil {
			return nil, err
		}
	}

	if msg.IsDeploy() {
		if err := execution.ValidateDeployMessage(msg); err != nil {
			return nil, err
		}
	}

	var payer execution.Payer
	if msg.IsInternal() || (emptyMessageIsRoot && args.Message == nil) {
		// "args.Message == nil" mean that it's a root message
		// and we don't want to withdraw any payment for it.
		// Because it's quite useful for read-only methods.
		payer = execution.NewMessagePayer(msg, es)
	} else {
		var toAs *execution.AccountState
		if toAs, err = es.GetAccount(msg.To); err != nil {
			return nil, err
		} else if toAs == nil {
			return nil, rpctypes.ErrToAccNotFound
		}
		payer = execution.NewAccountPayer(toAs, msg)
	}

	msgHash := es.AddInMessage(msg)
	res := es.HandleMessage(ctx, msg, payer)

	result := &rpctypes.CallResWithGasPrice{
		Data:      res.ReturnData,
		CoinsUsed: res.CoinsUsed,
	}

	if res.Failed() {
		result.Error = res.GetError().Error()
		return result, nil
	}

	esOld, err := execution.NewROExecutionState(tx, shardId, hash, timer, 1)
	if err != nil {
		return nil, err
	}
	stateOverrides, err := calculateStateChange(es, esOld)
	if err != nil {
		return nil, err
	}

	execOutMessages := es.OutMessages[msgHash]
	outMessages, err := api.handleOutMessages(
		ctx,
		execOutMessages,
		mainBlockHash,
		childBlocks,
		&stateOverrides,
	)
	if err != nil {
		return nil, err
	}

	result.OutMessages = outMessages
	result.StateOverrides = stateOverrides
	result.GasPrice = es.GasPrice
	return result, nil
}
