package jsonrpc

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
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
func (api *APIImpl) Call(ctx context.Context, args CallArgs, mainBlockNrOrHash transport.BlockNumberOrHash, overrides *StateOverrides) (*CallRes, error) {
	res, _, err := api.call(ctx, args, mainBlockNrOrHash, overrides, true)
	return res, err
}

func (api *APIImpl) call(
	ctx context.Context, args CallArgs, mainBlockNrOrHash transport.BlockNumberOrHash, overrides *StateOverrides, emptyMessageIsRoot bool,
) (*CallRes, types.Value, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, types.Value{}, err
	}
	defer tx.Rollback()

	msg, err := args.toMessage()
	if err != nil {
		return nil, types.Value{}, err
	}

	timer := common.NewTimer()
	shardId := msg.To.ShardId()

	mainBlock, err := api.fetchBlockByNumberOrHash(tx, types.MainShardId, mainBlockNrOrHash)
	if mainBlock == nil || err != nil {
		return nil, types.Value{}, err
	}

	treeShards := execution.NewDbShardBlocksTrieReader(tx, types.MainShardId, mainBlock.Id)
	treeShards.SetRootHash(mainBlock.ChildBlocksRootHash)
	hashBytes, err := treeShards.Get(shardId.Bytes())
	if err != nil {
		return nil, types.Value{}, err
	}

	hash := common.BytesToHash(hashBytes)
	es, err := execution.NewROExecutionState(tx, shardId, hash, timer, 1)
	if err != nil {
		return nil, types.Value{}, err
	}

	if overrides != nil {
		if err := overrides.Override(es); err != nil {
			return nil, types.Value{}, err
		}
	}

	if msg.IsDeploy() {
		if err := execution.ValidateDeployMessage(msg); err != nil {
			return nil, types.Value{}, err
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
			return nil, types.Value{}, err
		} else if toAs == nil {
			return nil, types.Value{}, ErrToAccNotFound
		}
		payer = execution.NewAccountPayer(toAs, msg)
	}

	msgHash := es.AddInMessage(msg)
	res := es.HandleMessage(ctx, msg, payer)

	result := &CallRes{
		Data:      res.ReturnData,
		CoinsUsed: res.CoinsUsed,
	}

	if res.Failed() {
		result.Error = res.GetError().Error()
		return result, types.Value{}, nil
	}

	esOld, err := execution.NewROExecutionState(tx, shardId, hash, timer, 1)
	if err != nil {
		return nil, types.Value{}, err
	}
	stateOverrides, err := calculateStateChange(es, esOld)
	if err != nil {
		return nil, types.Value{}, err
	}

	execOutMessages := es.OutMessages[msgHash]
	outMessages, err := api.handleOutMessages(
		ctx,
		execOutMessages,
		transport.BlockNumber(mainBlock.Id),
		&stateOverrides,
	)
	if err != nil {
		return nil, types.Value{}, err
	}

	result.OutMessages = outMessages
	result.StateOverrides = stateOverrides
	return result, es.GasPrice, nil
}

func feeIsNotEnough(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	return strings.HasPrefix(msg, vm.ErrOutOfGas.Error()) ||
		strings.HasPrefix(err.Error(), vm.ErrInsufficientBalance.Error())
}

// Add some gap (20%) to be sure that it's enough for message processing.
// For now it's just heuristic function without any mathematical rationality.
func refineResult(input types.Value) types.Value {
	return input.Mul64(12).Div64(10)
}

// Call implements eth_estimateGas.
func (api *APIImpl) EstimateFee(ctx context.Context, args CallArgs, mainBlockNrOrHash transport.BlockNumberOrHash) (types.Value, error) {
	gasCap := types.NewValueFromUint64(100_000_000)
	feeCreditCap := types.NewValueFromUint64(50_000_000)

	execute := func(balance, feeCredit types.Value) (*CallRes, types.Value, error) {
		args.FeeCredit = feeCredit

		stateOverrides := &StateOverrides{
			args.To: Contract{
				Balance: &balance,
			},
		}

		// Root message considered here as external since we anyway override contract balance.
		res, gasPrice, err := api.call(ctx, args, mainBlockNrOrHash, stateOverrides, false)
		if err != nil {
			return nil, types.Value{}, err
		}

		if res.Error != "" {
			return nil, types.Value{}, errors.New(res.Error)
		}
		return res, gasPrice, nil
	}

	// Check that it's possible to run transaction with Max balance and feeCredit
	res, gasPrice, err := execute(gasCap, feeCreditCap)
	if err != nil {
		return types.Value{}, err
	}

	var lo, hi types.Value = res.CoinsUsed.Add(args.Value), feeCreditCap

	// Binary search implementation.
	// Some opcodes can require some gas reserve (so we can't use coinsUsed).
	// As result we try to find minimal value then call works successful.
	// E.g. see how "SstoreSentryGasEIP2200" is used.
	for lo.Add64(1).Int().Lt(hi.Int()) {
		mid := hi.Add(lo).Div64(2)

		_, _, err = execute(gasCap, mid)
		switch {
		case err == nil:
			hi = mid
		case feeIsNotEnough(err):
			lo = mid
		default:
			return types.Value{}, err
		}
	}
	if err != nil && !feeIsNotEnough(err) {
		return types.Value{}, err
	}

	result := hi
	if !args.Flags.GetBit(types.MessageFlagInternal) {
		// Heuristic price for external message verification for the wallet.
		const externalVerificationGas = types.Gas(10_000)
		result = result.Add(externalVerificationGas.ToValue(gasPrice))
	}
	return refineResult(result), nil
}
