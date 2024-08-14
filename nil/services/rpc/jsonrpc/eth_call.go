package jsonrpc

import (
	"bytes"
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

func calculateStateChange(newEs, oldEs *execution.ExecutionState) (StateOverrides, error) {
	stateOverrides := make(StateOverrides)

	for addr, as := range newEs.Accounts {
		var contract Contract
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

func (api *APIImpl) handleOutMessages(
	ctx context.Context,
	outMsgs []*types.OutboundMessage,
	mainblockNum transport.BlockNumber,
	overrides *StateOverrides,
) ([]*OutMessage, error) {
	outMessages := make([]*OutMessage, len(outMsgs))

	for i, outMsg := range outMsgs {
		raw, err := outMsg.Message.MarshalSSZ()
		if err != nil {
			return nil, err
		}

		args := CallArgs{
			Message: (*hexutil.Bytes)(&raw),
		}

		res, err := api.Call(ctx, args, transport.BlockNumberOrHash{BlockNumber: &mainblockNum}, overrides)
		if err != nil {
			return nil, err
		}

		outMessages[i] = &OutMessage{
			Message:     outMsg,
			Data:        res.Data,
			CoinsUsed:   res.CoinsUsed,
			OutMessages: res.OutMessages,
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

// Call implements eth_call. Executes a new message call immediately without creating a transaction on the block chain.
func (api *APIImpl) Call(ctx context.Context, args CallArgs, mainblockNrOrHash transport.BlockNumberOrHash, overrides *StateOverrides) (*CallRes, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	msg, err := args.toMessage()
	if err != nil {
		return nil, err
	}

	timer := common.NewTimer()
	shardId := msg.To.ShardId()

	mainBlock, err := api.fetchBlockByNumberOrHash(tx, types.MainShardId, mainblockNrOrHash)
	if mainBlock == nil || err != nil {
		return nil, err
	}

	treeShards := execution.NewDbShardBlocksTrieReader(tx, types.MainShardId, mainBlock.Id)
	treeShards.SetRootHash(mainBlock.ChildBlocksRootHash)
	entries, err := treeShards.Entries()
	if err != nil {
		return nil, err
	}

	shardIdToHash := make(map[types.ShardId]common.Hash)
	for _, e := range entries {
		shardIdToHash[e.Key] = *e.Val
	}

	hash, exists := shardIdToHash[shardId]
	if !exists {
		return nil, ErrShardNotFound
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
	if args.Message == nil || msg.IsInternal() {
		// "args.Message == nil" mean that it's a root message
		// and we don't want to withdraw any payment for it.
		// Because it's quite useful for read-only methods.
		payer = execution.NewMessagePayer(msg, es)
	} else {
		var toAs *execution.AccountState
		if toAs, err = es.GetAccount(msg.To); err != nil {
			return nil, err
		} else if toAs == nil {
			return nil, ErrToAccNotFound
		}
		payer = execution.NewAccountPayer(toAs, msg)
	}

	es.AddInMessage(msg)
	res := es.HandleMessage(ctx, msg, payer)

	result := &CallRes{
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

	execOutMessages := es.OutMessages[msg.Hash()]
	outMessages, err := api.handleOutMessages(
		ctx,
		execOutMessages,
		transport.BlockNumber(mainBlock.Id),
		&stateOverrides,
	)
	if err != nil {
		return nil, err
	}

	result.OutMessages = outMessages
	result.StateOverrides = stateOverrides
	return result, nil
}
