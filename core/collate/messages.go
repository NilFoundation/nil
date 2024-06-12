package collate

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/tracing"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/holiman/uint256"
)

type payer interface {
	fmt.Stringer
	CanPay(*big.Int) bool
	SubBalance(*uint256.Int)
	AddBalance(*uint256.Int)
}

type messagePayer struct {
	message *types.Message
}

func (m messagePayer) CanPay(amount *big.Int) bool {
	return m.message.Value.Int.ToBig().Cmp(amount) >= 0
}

func (m messagePayer) SubBalance(delta *uint256.Int) {
	m.message.Value.Sub(&m.message.Value.Int, delta)
}

func (m messagePayer) AddBalance(delta *uint256.Int) {
	m.message.Value.Add(&m.message.Value.Int, delta)
}

func (m messagePayer) String() string {
	return "message"
}

type accountPayer struct {
	account *execution.AccountState
	message *types.Message
}

func (a accountPayer) CanPay(amount *big.Int) bool {
	required := a.message.Value.Int.ToBig()
	required.Add(required, amount)
	return a.account.Balance.ToBig().Cmp(required) >= 0
}

func (a accountPayer) SubBalance(amount *uint256.Int) {
	a.account.SubBalance(amount, tracing.BalanceDecreaseGasBuy)
}

func (a accountPayer) AddBalance(amount *uint256.Int) {
	a.account.AddBalance(amount, tracing.BalanceIncreaseGasReturn)
}

func (a accountPayer) String() string {
	return fmt.Sprintf("account %v", a.message.From.Hex())
}

func HandleMessages(ctx context.Context, roTx db.RoTx, es *execution.ExecutionState, msgs []*types.Message) error {
	blockContext := execution.NewEVMBlockContext(es)
	for _, message := range msgs {
		msgHash := message.Hash()
		es.AddInMessage(message)
		es.InMessageHash = msgHash

		ok, err := validateMessage(roTx, es, message)
		if err != nil {
			return err
		}
		if !ok {
			addFailReceipt(es, errors.New("validateMessage failed"))
			continue
		}

		var payer payer
		if message.Internal {
			payer = messagePayer{message}
		} else {
			payer = accountPayer{es.GetAccount(message.From), message}
		}
		err = buyGas(payer, message)
		if err != nil {
			sharedLogger.Info().Err(err).Stringer("hash", es.InMessageHash).Msg("discarding message")
			continue
		}
		var leftOverGas uint64

		// Deploy message
		if message.Deploy {
			deployMsg := validateDeployMessage(es, message)
			if deployMsg == nil {
				continue
			}

			if leftOverGas, err = es.HandleDeployMessage(ctx, message, deployMsg, &blockContext); err != nil && !errors.Is(err, new(vm.VMError)) {
				return err
			}
		} else {
			if leftOverGas, _, err = es.HandleExecutionMessage(ctx, message, &blockContext); err != nil && !errors.Is(err, new(vm.VMError)) {
				return err
			}
		}
		refundGas(payer, message, leftOverGas)
	}

	return nil
}

var (
	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrInsufficientFunds = errors.New("insufficient funds for gas * price + value")

	// ErrGasUintOverflow is returned when calculating gas usage.
	ErrGasUintOverflow = errors.New("gas uint64 overflow")
)

func buyGas(payer payer, message *types.Message) error {
	mgval := message.GasLimit.ToBig()
	mgval.Mul(mgval, message.GasPrice.ToBig())

	required, overflow := uint256.FromBig(mgval)
	if overflow {
		return fmt.Errorf("%w: %v required balance exceeds 256 bits", ErrInsufficientFunds, payer.String())
	}
	if !payer.CanPay(mgval) {
		return fmt.Errorf("%w: %v can't pay %v", ErrInsufficientFunds, payer.String(), required)
	}

	payer.SubBalance(required)
	return nil
}

func refundGas(payer payer, message *types.Message, gasRemaining uint64) {
	// Return currency for remaining gas, exchanged at the original rate.
	remaining := uint256.NewInt(gasRemaining)
	remaining.Mul(remaining, &message.GasPrice.Int)
	payer.AddBalance(remaining)
}

func validateDeployMessage(es *execution.ExecutionState, message *types.Message) *types.DeployMessage {
	fail := func(err error, message string) *types.DeployMessage {
		addFailReceipt(es, err)
		sharedLogger.Debug().Err(err).Stringer(logging.FieldMessageHash, es.InMessageHash).Msg(message)
		return nil
	}

	deployMsg, err := types.NewDeployMessage(message.Data)
	if err != nil {
		return fail(err, "Invalid deploy message")
	}

	if types.IsMasterShard(deployMsg.ShardId) && message.From != types.MainWalletAddress {
		return fail(nil, "Attempt to deploy to master shard from non system wallet")
	}

	if message.To != types.CreateAddress(message.To.ShardId(), deployMsg.Code) {
		return fail(nil, "Incorrect deployment address")
	}

	return deployMsg
}

func validateMessage(roTx db.RoTx, es *execution.ExecutionState, message *types.Message) (bool, error) {
	if message.Internal {
		fromId := message.From.ShardId()
		data, err := es.Accessor.Access(roTx, fromId).GetOutMessage().ByHash(message.Hash())
		if err != nil {
			addFailReceipt(es, err)
			return false, err
		}
		return data.Message() != nil, nil
	}

	addr := message.To
	r := &types.Receipt{
		Success:         false,
		GasUsed:         0,
		MsgHash:         es.InMessageHash,
		ContractAddress: addr,
	}

	accountState := es.GetAccount(addr)
	if accountState == nil {
		r.Logs = es.Logs[es.InMessageHash]
		es.AddReceipt(r)
		sharedLogger.Debug().
			Stringer(logging.FieldShardId, es.ShardId).
			Stringer(logging.FieldMessageFrom, addr).
			Msg("Invalid address.")
		return false, nil
	}

	var ok bool
	if es.ContractExists(addr) {
		ok = es.CallVerifyExternal(message, accountState)
	} else {
		// External deployment. Ensure that the account pays for it itself
		ok = (message.From == message.To)
	}

	if !ok {
		r.Logs = es.Logs[es.InMessageHash]
		es.AddReceipt(r)
		sharedLogger.Debug().
			Stringer(logging.FieldShardId, es.ShardId).
			Stringer(logging.FieldMessageFrom, addr).
			Msg("Invalid signature.")
		return false, nil
	}

	if accountState.Seqno != message.Seqno {
		r.Logs = es.Logs[es.InMessageHash]
		es.AddReceipt(r)
		sharedLogger.Debug().
			Stringer(logging.FieldShardId, es.ShardId).
			Stringer(logging.FieldMessageFrom, addr).
			Uint64(logging.FieldAccountSeqno, accountState.Seqno.Uint64()).
			Uint64(logging.FieldMessageSeqno, message.Seqno.Uint64()).
			Msg("Seqno gap")
		return false, nil
	}

	return true, nil
}

func addFailReceipt(es *execution.ExecutionState, err error) {
	r := &types.Receipt{
		Success: false,
		MsgHash: es.InMessageHash,
	}
	es.AddReceipt(r)
	sharedLogger.Error().Err(err).Stringer("hash", es.InMessageHash).Msg("Add fail receipt")
}
