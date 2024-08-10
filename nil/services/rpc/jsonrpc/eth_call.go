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

// Call implements eth_call. Executes a new message call immediately without creating a transaction on the block chain.
func (api *APIImpl) Call(ctx context.Context, args CallArgs, mainblockNrOrHash transport.BlockNumberOrHash, overrides *StateOverrides) (*CallRes, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	timer := common.NewTimer()
	shardId := args.To.ShardId()

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
	shardIdToEs := make(map[types.ShardId]*execution.ExecutionState)

	for _, e := range entries {
		shardIdToHash[e.Key] = *e.Val
	}

	getShardExecutionState := func(shardId types.ShardId) (*execution.ExecutionState, error) {
		es, exists := shardIdToEs[shardId]
		if exists {
			return es, nil
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

		shardIdToEs[shardId] = es
		return es, nil
	}

	es, err := getShardExecutionState(shardId)
	if err != nil {
		return nil, err
	}

	msg := &types.Message{
		ChainId:   types.DefaultChainId,
		Seqno:     args.Seqno,
		FeeCredit: args.FeeCredit,
		To:        args.To,
		Value:     args.Value,
		Data:      types.Code(args.Data),
	}

	var fromAs *execution.AccountState
	if args.From != nil {
		msg.From = *args.From
		msg.Flags.SetBit(types.MessageFlagInternal)

		if fromAs, err = es.GetAccount(*args.From); err != nil {
			return nil, err
		} else if fromAs == nil {
			return nil, ErrFromAccNotFound
		}
	} else {
		msg.From = msg.To
	}

	if as, err := es.GetAccount(args.To); err != nil {
		return nil, err
	} else if as == nil {
		msg.Flags.SetBit(types.MessageFlagDeploy)
	}

	if msg.IsDeploy() {
		if err := execution.ValidateDeployMessage(msg); err != nil {
			return nil, err
		}
	}

	var payer execution.Payer
	if msg.IsInternal() {
		payer = execution.NewAccountPayer(fromAs, msg)
	} else {
		payer = execution.NewMessagePayer(msg, es)
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

	var handleOutMessages func(outMsgs []*types.OutboundMessage) ([]*OutMessage, error)
	handleOutMessages = func(outMsgs []*types.OutboundMessage) ([]*OutMessage, error) {
		outMessages := make([]*OutMessage, len(outMsgs))

		for i, outMsg := range outMsgs {
			msg := outMsg.Message

			es, err := getShardExecutionState(msg.To.ShardId())
			if err != nil {
				return nil, err
			}

			es.AddInMessage(msg)
			payer := execution.NewMessagePayer(msg, es)
			execRes := es.HandleMessage(ctx, msg, payer)

			outMessages[i] = &OutMessage{
				Message:   outMsg,
				Data:      execRes.ReturnData,
				CoinsUsed: execRes.CoinsUsed,
			}

			if !execRes.Failed() {
				outMessages[i].OutMessages, err = handleOutMessages(es.OutMessages[msg.Hash()])
				if err != nil {
					return nil, err
				}
			} else {
				outMessages[i].Error = execRes.GetError().Error()
			}
		}

		return outMessages, nil
	}

	execOutMessages := es.OutMessages[msg.Hash()]
	outMessages, err := handleOutMessages(execOutMessages)
	if err != nil {
		return nil, err
	}

	// Calculate state diff between original state and result
	stateOverrides := make(StateOverrides)
	for shardId, esNew := range shardIdToEs {
		hash := shardIdToHash[shardId]
		esOld, err := execution.NewROExecutionState(tx, shardId, hash, timer, 1)
		if err != nil {
			return nil, err
		}
		shardStateOverrides, err := calculateStateChange(esNew, esOld)
		if err != nil {
			return nil, err
		}
		for k, v := range shardStateOverrides {
			stateOverrides[k] = v
		}
	}

	result.OutMessages = outMessages
	result.StateOverrides = stateOverrides
	return result, nil
}
