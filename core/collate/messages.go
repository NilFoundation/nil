package collate

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/common/check"
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
	es      *execution.ExecutionState
}

func (m messagePayer) CanPay(amount *big.Int) bool {
	return m.message.Value.Int.ToBig().Cmp(amount) >= 0
}

func (m messagePayer) SubBalance(delta *uint256.Int) {
	m.message.Value.Sub(&m.message.Value.Int, delta)
}

func (m messagePayer) AddBalance(delta *uint256.Int) {
	if m.message.RefundTo == types.EmptyAddress {
		sharedLogger.Error().Stringer(logging.FieldMessageHash, m.message.Hash()).Msg("refund address is empty")
		return
	}
	m.es.AddOutMessageForTx(m.message.Hash(), &types.Message{
		Internal: true,
		Kind:     types.RefundMessageKind,
		From:     m.message.To,
		To:       m.message.RefundTo,
		Value:    types.Uint256{Int: *delta},
	})
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
	blockContext, err := execution.NewEVMBlockContext(es)
	if err != nil {
		return err
	}

	for _, inMessage := range msgs {
		msgHash := inMessage.Hash()
		es.AddInMessage(inMessage)
		es.InMessageHash = msgHash

		// We copy the message so as not to spoil es.InMessages when purchasing gas
		inMessageCopy := *inMessage
		message := &inMessageCopy

		ok, payer := validateMessage(roTx, es, message)
		if !ok {
			continue
		}
		err := buyGas(payer, message)
		if err != nil {
			sharedLogger.Info().Err(err).Stringer("hash", es.InMessageHash).Msg("discarding message")
			es.AddReceipt(&types.Receipt{Success: false, ContractAddress: message.To, MsgHash: msgHash})
			continue
		}
		var leftOverGas uint64

		switch message.Kind {
		case types.DeployMessageKind:
			deployMsg := validateDeployMessage(es, message)
			if deployMsg == nil {
				es.AddReceipt(&types.Receipt{Success: false, ContractAddress: message.To, MsgHash: msgHash})
				continue
			}

			if leftOverGas, err = es.HandleDeployMessage(ctx, message, deployMsg, blockContext); err != nil && !errors.As(err, new(vm.VMError)) {
				return err
			}
			refundGas(payer, message, leftOverGas)
		case types.ExecutionMessageKind:
			if leftOverGas, _, err = es.HandleExecutionMessage(ctx, message, blockContext); err != nil && !errors.As(err, new(vm.VMError)) {
				return err
			}
			refundGas(payer, message, leftOverGas)
		case types.RefundMessageKind:
			es.HandleRefundMessage(ctx, message)
		default:
			panic("unreachable")
		}
	}

	return nil
}

var (
	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrInsufficientFunds = errors.New("insufficient funds for gas * price + value")

	// ErrGasUintOverflow is returned when calculating gas usage.
	ErrGasUintOverflow = errors.New("gas uint64 overflow")

	// ErrInternalMessageValidationFailed is returned when no corresponding outgoing message is found.
	ErrInternalMessageValidationFailed = errors.New("internal message validation failed")

	// ErrNoPayer is returned when no account at address specified to pay fees.
	ErrNoPayer = errors.New("no account at address to pay fees")

	// ErrContractAlreadyExists is returned when attempt to deploy code to address of already deployed contract.
	ErrContractAlreadyExists = errors.New("contract already exists")

	// ErrContractDoesNotExists is returned when attempt to call non-existent contract.
	ErrContractDoesNotExist = errors.New("contract does not exist")

	// ErrSeqnoGap is returned when message seqno does not match the seqno of the recipient.
	ErrSeqnoGap = errors.New("seqno gap")

	// ErrInvalidSignature is returned when verifyExternal call fails.
	ErrInvalidSignature = errors.New("invalid signature")

	// ErrInvalidChainId is returned when message chain id is different from DefaultChainId.
	ErrInvalidChainId = errors.New("invalid chainId")
)

// TODO: Make gas price dynamic and use message.GasPrice
var GasPrice = uint256.NewInt(10)

func buyGas(payer payer, message *types.Message) error {
	mgval := message.GasLimit.ToBig()
	mgval.Mul(mgval, GasPrice.ToBig())

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

func refundGas(payer payer, _ *types.Message, gasRemaining uint64) {
	// Return currency for remaining gas, exchanged at the original rate.
	remaining := uint256.NewInt(gasRemaining)
	remaining.Mul(remaining, GasPrice)
	payer.AddBalance(remaining)
}

func validateDeployMessage(es *execution.ExecutionState, message *types.Message) *types.DeployPayload {
	address := message.To
	fail := func(err error, message string) *types.DeployPayload {
		addFailReceipt(es, address, err)
		sharedLogger.Debug().Err(err).Stringer(logging.FieldMessageHash, es.InMessageHash).Msg(message)
		return nil
	}

	deployPayload := types.ParseDeployPayload(message.Data)
	if deployPayload == nil {
		return fail(nil, "Invalid deploy payload")
	}

	if types.IsMasterShard(address.ShardId()) && message.From != types.MainWalletAddress {
		return fail(nil, "Attempt to deploy to master shard from non system wallet")
	}

	if message.To != types.CreateAddress(address.ShardId(), deployPayload.Bytes()) {
		return fail(nil, "Incorrect deployment address")
	}

	return deployPayload
}

func validateInternalMessage(roTx db.RoTx, es *execution.ExecutionState, message *types.Message) error {
	check.PanicIfNot(message.Internal)

	fromId := message.From.ShardId()
	data, err := es.Accessor.Access(roTx, fromId).GetOutMessage().ByHash(message.Hash())
	if err != nil {
		return err
	}
	if data.Message() == nil {
		return ErrInternalMessageValidationFailed
	}
	return nil
}

func validateExternalDeployMessage(es *execution.ExecutionState, message *types.Message) error {
	check.PanicIfNot(!message.Internal)
	check.PanicIfNot(message.Kind == types.DeployMessageKind)

	addr := message.To
	accountState := es.GetAccount(addr)
	if accountState == nil {
		err := ErrNoPayer
		sharedLogger.Debug().
			Err(ErrNoPayer).
			Stringer("hash", es.InMessageHash).
			Stringer(logging.FieldShardId, es.ShardId).
			Stringer(logging.FieldMessageFrom, addr).
			Send()
		return err
	}

	if es.ContractExists(addr) {
		return ErrContractAlreadyExists
	}

	return nil
}

func validateExternalExecutionMessage(es *execution.ExecutionState, message *types.Message) error {
	check.PanicIfNot(!message.Internal)
	check.PanicIfNot(message.Kind == types.ExecutionMessageKind)

	addr := message.To
	if !es.ContractExists(addr) {
		if len(message.Data) > 0 && message.Value.IsZero() {
			return ErrContractDoesNotExist
		}
		return nil // Just send value
	}

	accountState := es.GetAccount(addr)
	check.PanicIfNot(accountState != nil)

	if accountState.Seqno != message.Seqno {
		err := ErrSeqnoGap
		sharedLogger.Debug().
			Err(err).
			Stringer("hash", es.InMessageHash).
			Stringer(logging.FieldShardId, es.ShardId).
			Stringer(logging.FieldMessageFrom, addr).
			Uint64(logging.FieldAccountSeqno, accountState.Seqno.Uint64()).
			Uint64(logging.FieldMessageSeqno, message.Seqno.Uint64()).
			Send()
		return err
	}

	ok, err := es.CallVerifyExternal(message, accountState)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidSignature
	}
	return nil
}

func validateExternalMessage(es *execution.ExecutionState, message *types.Message) error {
	if message.ChainId != types.DefaultChainId {
		err := ErrInvalidChainId
		sharedLogger.Debug().
			Err(err).
			Uint64(logging.FieldChainId, uint64(message.ChainId)).
			Send()
		return err
	}

	switch message.Kind {
	case types.DeployMessageKind:
		return validateExternalDeployMessage(es, message)
	case types.ExecutionMessageKind:
		return validateExternalExecutionMessage(es, message)
	case types.RefundMessageKind:
		return errors.New("refund message is not allowed in external messages")
	default:
		panic("unreachable")
	}
}

func validateMessage(roTx db.RoTx, es *execution.ExecutionState, message *types.Message) (bool, payer) {
	if message.Internal {
		if err := validateInternalMessage(roTx, es, message); err != nil {
			addFailReceipt(es, message.To, err)
			return false, nil
		}
		return true, messagePayer{message, es}
	}

	if err := validateExternalMessage(es, message); err != nil {
		// TODO: Inform RPC about errors without storing receipts in the blockchain
		addFailReceipt(es, message.To, err)
		return false, nil
	}
	return true, accountPayer{es.GetAccount(message.To), message}
}

func addFailReceipt(es *execution.ExecutionState, address types.Address, err error) {
	r := &types.Receipt{
		Success:         false,
		MsgHash:         es.InMessageHash,
		Logs:            es.Logs[es.InMessageHash],
		ContractAddress: address,
	}
	es.AddReceipt(r)
	sharedLogger.Error().Err(err).Stringer("hash", es.InMessageHash).Msg("Add fail receipt")
}
