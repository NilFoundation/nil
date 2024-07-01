package execution

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/tracing"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
)

var sharedLogger = logging.NewLogger("execution")

type payer interface {
	fmt.Stringer
	CanPay(*big.Int) bool
	SubBalance(*uint256.Int)
	AddBalance(*uint256.Int)
}

type messagePayer struct {
	message *types.Message
	es      *ExecutionState
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
	account *AccountState
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

func buyGas(payer payer, message *types.Message, gasPrice *uint256.Int) error {
	mgval := message.GasLimit.ToBig()
	mgval.Mul(mgval, gasPrice.ToBig())

	required, overflow := uint256.FromBig(mgval)
	if overflow {
		return fmt.Errorf("%w: %s required balance exceeds 256 bits", ErrInsufficientFunds, payer)
	}
	if !payer.CanPay(mgval) {
		return fmt.Errorf("%w: %s can't pay %v", ErrInsufficientFunds, payer, required)
	}

	payer.SubBalance(required)
	return nil
}

func refundGas(payer payer, _ *types.Message, gasRemaining uint64, gasPrice *uint256.Int) {
	// Return currency for remaining gas, exchanged at the original rate.
	remaining := uint256.NewInt(gasRemaining)
	remaining.Mul(remaining, gasPrice)
	payer.AddBalance(remaining)
}

func ValidateDeployMessage(message *types.Message) error {
	deployPayload := types.ParseDeployPayload(message.Data)
	if deployPayload == nil {
		return ErrInvalidPayload
	}

	shardId := message.To.ShardId()
	if types.IsMasterShard(shardId) && message.From != types.MainWalletAddress {
		return ErrDeployToMainShard
	}

	if message.To != types.CreateAddress(shardId, *deployPayload) {
		return ErrIncorrectDeploymentAddress
	}

	return nil
}

func validateExternalDeployMessage(es *ExecutionState, message *types.Message) error {
	check.PanicIfNot(message.Kind == types.DeployMessageKind)

	if err := ValidateDeployMessage(message); err != nil {
		return err
	}

	if exists, err := es.ContractExists(message.To); err != nil {
		return err
	} else if exists {
		return ErrContractAlreadyExists
	}

	return nil
}

func validateExternalExecutionMessage(es *ExecutionState, message *types.Message, gasPrice *uint256.Int) error {
	check.PanicIfNot(message.Kind == types.ExecutionMessageKind)

	to := message.To
	if exists, err := es.ContractExists(to); err != nil {
		return err
	} else if !exists {
		if len(message.Data) > 0 && message.Value.IsZero() {
			return ErrContractDoesNotExist
		}
		return nil // Just send value
	}

	account, err := es.GetAccount(to)
	check.PanicIfErr(err)
	if account.ExtSeqno != message.Seqno {
		return fmt.Errorf("%w: account %v != message %v", ErrSeqnoGap, account.ExtSeqno, message.Seqno)
	}

	ok, err := es.CallVerifyExternal(message, account, gasPrice)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidSignature
	}
	return nil
}

func ValidateExternalMessage(es *ExecutionState, message *types.Message, gasPrice *uint256.Int) error {
	check.PanicIfNot(!message.Internal)

	if message.ChainId != types.DefaultChainId {
		return ErrInvalidChainId
	}

	if account, err := es.GetAccount(message.To); err != nil {
		return err
	} else if account == nil {
		return ErrNoPayer
	}

	switch message.Kind {
	case types.DeployMessageKind:
		return validateExternalDeployMessage(es, message)
	case types.ExecutionMessageKind:
		return validateExternalExecutionMessage(es, message, gasPrice)
	case types.RefundMessageKind:
		return errors.New("refund message is not allowed in external messages")
	default:
		panic("unreachable")
	}
}
