package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
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
func (api *APIImpl) Call(ctx context.Context, args CallArgs, blockNrOrHash transport.BlockNumberOrHash, overrides *StateOverrides) (*CallRes, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	timer := common.NewTimer()
	shardId := args.From.ShardId()

	hash, err := extractBlockHash(tx, shardId, blockNrOrHash)
	if err != nil {
		return nil, err
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
		From:      args.From,
		To:        args.To,
		Value:     args.Value,
		Data:      types.Code(args.Data),
	}

	es.AddInMessage(msg)
	res := es.HandleMessage(ctx, msg, execution.SimPayer{})
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
