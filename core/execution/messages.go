package execution

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/tracing"
	"github.com/NilFoundation/nil/core/types"
)

var sharedLogger = logging.NewLogger("execution")

type payer interface {
	fmt.Stringer
	CanPay(types.Value) bool
	SubBalance(types.Value)
	AddBalance(types.Value)
}

type messagePayer struct {
	message *types.Message
	es      *ExecutionState
}

func (m messagePayer) CanPay(amount types.Value) bool {
	return m.message.Value.Cmp(amount) >= 0
}

func (m messagePayer) SubBalance(delta types.Value) {
	m.message.Value = m.message.Value.Sub(delta)
}

func (m messagePayer) AddBalance(delta types.Value) {
	if m.message.RefundTo == types.EmptyAddress {
		sharedLogger.Error().Stringer(logging.FieldMessageHash, m.message.Hash()).Msg("refund address is empty")
		return
	}
	m.es.AppendOutMessage(&types.Message{
		Flags: types.NewMessageFlags(types.MessageFlagInternal, types.MessageFlagRefund),
		From:  m.message.To,
		To:    m.message.RefundTo,
		Value: delta,
	})
}

func (m messagePayer) String() string {
	return "message"
}

type accountPayer struct {
	account *AccountState
	message *types.Message
}

func (a accountPayer) CanPay(amount types.Value) bool {
	return a.account.Balance.Cmp(a.message.Value.Add(amount)) >= 0
}

func (a accountPayer) SubBalance(amount types.Value) {
	a.account.SubBalance(amount, tracing.BalanceDecreaseGasBuy)
}

func (a accountPayer) AddBalance(amount types.Value) {
	a.account.AddBalance(amount, tracing.BalanceIncreaseGasReturn)
}

func (a accountPayer) String() string {
	return fmt.Sprintf("account %v", a.message.From.Hex())
}

func buyGas(payer payer, message *types.Message, gasPrice types.Value) error {
	required, overflow := message.GasLimit.ToValueOverflow(gasPrice)
	if overflow {
		return fmt.Errorf("%w: %s required balance exceeds 256 bits", ErrInsufficientFunds, payer)
	}
	if !payer.CanPay(required) {
		return fmt.Errorf("%w: %s can't pay %s", ErrInsufficientFunds, payer, required)
	}

	payer.SubBalance(required)
	return nil
}

func refundGas(payer payer, _ *types.Message, gasRemaining types.Gas, gasPrice types.Value) {
	if gasRemaining == 0 {
		return
	}
	// Return currency for remaining gas, exchanged at the original rate.
	payer.AddBalance(gasRemaining.ToValue(gasPrice))
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
	check.PanicIfNot(message.IsDeploy())

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

func validateExternalExecutionMessage(es *ExecutionState, message *types.Message, gasPrice types.Value) error {
	check.PanicIfNot(message.IsExecution())

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

func ValidateExternalMessage(es *ExecutionState, message *types.Message, gasPrice types.Value) error {
	check.PanicIfNot(message.IsExternal())

	if message.ChainId != types.DefaultChainId {
		return ErrInvalidChainId
	}

	if account, err := es.GetAccount(message.To); err != nil {
		return err
	} else if account == nil {
		return ErrNoPayer
	}

	switch {
	case message.IsDeploy():
		return validateExternalDeployMessage(es, message)
	case message.IsRefund():
		return errors.New("refund message is not allowed in external messages")
	default:
		return validateExternalExecutionMessage(es, message, gasPrice)
	}
}
