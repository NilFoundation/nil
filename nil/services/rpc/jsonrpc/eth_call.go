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

	var hash common.Hash
	for _, e := range entries {
		if e.Key == shardId {
			hash = *e.Val
		}
	}

	if hash == common.EmptyHash {
		return nil, nil
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
		// msg.Flags.SetBit(types.MessageFlagInternal) // TODO: execution hangs if enabled

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
		payer = &execution.SimPayer{}
	}

	es.AddInMessage(msg)
	res := es.HandleMessage(ctx, msg, payer)
	if res.IsFatal() {
		return nil, res.FatalError
	}
	if res.Error != nil {
		return nil, res.Error.Unwrap()
	}
	outMessages := es.OutMessages[msg.Hash()]

	// Calculate state diff between original state and result
	esOld, err := execution.NewROExecutionState(tx, shardId, hash, timer, 1)
	if err != nil {
		return nil, err
	}
	stateOverrides, err := calculateStateChange(es, esOld)
	if err != nil {
		return nil, err
	}

	return &CallRes{
		Data:           res.ReturnData,
		CoinsUsed:      res.CoinsUsed,
		OutMessages:    outMessages,
		StateOverrides: stateOverrides,
	}, nil
}
