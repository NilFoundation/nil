package execution

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
)

var sharedLogger = logging.NewLogger("execution")

type Payer interface {
	fmt.Stringer
	CanPay(types.Value) bool
	SubBalance(types.Value)
	AddBalance(types.Value)
}

type messagePayer struct {
	message *types.Message
	es      *ExecutionState
}

func NewMessagePayer(message *types.Message, es *ExecutionState) messagePayer {
	return messagePayer{
		message: message,
		es:      es,
	}
}

func (m messagePayer) CanPay(amount types.Value) bool {
	return true
}

func (m messagePayer) SubBalance(_ types.Value) {
	// Already paid by sender
}

func (m messagePayer) AddBalance(delta types.Value) {
	if m.message.RefundTo == types.EmptyAddress {
		sharedLogger.Error().Stringer(logging.FieldMessageHash, m.message.Hash()).Msg("refund address is empty")
		return
	}

	if _, err := m.es.AddOutInternal(m.message.To, &types.InternalMessagePayload{
		Kind:  types.RefundMessageKind,
		To:    m.message.RefundTo,
		Value: delta,
	}); err != nil {
		sharedLogger.Error().Err(err).Stringer(logging.FieldMessageHash, m.message.Hash()).Msg("failed to add refund message")
	}
}

func (m messagePayer) String() string {
	return "message"
}

func NewAccountPayer(account *AccountState, message *types.Message) accountPayer {
	return accountPayer{
		account: account,
		message: message,
	}
}

type accountPayer struct {
	account *AccountState
	message *types.Message
}

func (a accountPayer) CanPay(amount types.Value) bool {
	value, overflow := a.message.Value.AddOverflow(amount)
	check.PanicIfNot(!overflow)
	return a.account.Balance.Cmp(value) >= 0
}

func (a accountPayer) SubBalance(amount types.Value) {
	check.PanicIfErr(a.account.SubBalance(amount, tracing.BalanceDecreaseGasBuy))
}

func (a accountPayer) AddBalance(amount types.Value) {
	check.PanicIfErr(a.account.AddBalance(amount, tracing.BalanceIncreaseGasReturn))
}

func (a accountPayer) String() string {
	return fmt.Sprintf("account %v", a.message.From.Hex())
}

func buyGas(payer Payer, message *types.Message) error {
	if !payer.CanPay(message.FeeCredit) {
		return fmt.Errorf("%w: %s can't pay %s", ErrInsufficientFunds, payer, message.FeeCredit)
	}
	payer.SubBalance(message.FeeCredit)
	return nil
}

func refundGas(payer Payer, gasRemaining types.Value) {
	if gasRemaining.IsZero() {
		return
	}
	// Return currency for remaining gas, exchanged at the original rate.
	payer.AddBalance(gasRemaining)
}

func ValidateDeployMessage(message *types.Message) error {
	deployPayload := types.ParseDeployPayload(message.Data)
	if deployPayload == nil {
		return ErrInvalidPayload
	}

	shardId := message.To.ShardId()
	if shardId.IsMainShard() {
		return ErrDeployToMainShard
	}

	if message.To != types.CreateAddress(shardId, *deployPayload) {
		return ErrIncorrectDeploymentAddress
	}

	return nil
}

func validateExternalDeployMessage(es *ExecutionState, message *types.Message) *ExecutionResult {
	check.PanicIfNot(message.IsDeploy())

	if err := ValidateDeployMessage(message); err != nil {
		return NewExecutionResult().SetError(NewMessageError(err))
	}

	if exists, err := es.ContractExists(message.To); err != nil {
		return NewExecutionResult().SetFatal(err)
	} else if exists {
		return NewExecutionResult().SetError(NewMessageError(ErrContractAlreadyExists))
	}

	return NewExecutionResult()
}

func validateExternalExecutionMessage(es *ExecutionState, message *types.Message) *ExecutionResult {
	check.PanicIfNot(message.IsExecution())

	to := message.To
	if exists, err := es.ContractExists(to); err != nil {
		return NewExecutionResult().SetFatal(err)
	} else if !exists {
		if len(message.Data) > 0 && message.Value.IsZero() {
			return NewExecutionResult().SetError(NewMessageError(ErrContractDoesNotExist))
		}
		return NewExecutionResult() // send value
	}

	account, err := es.GetAccount(to)
	check.PanicIfErr(err)
	if account.ExtSeqno != message.Seqno {
		err := fmt.Errorf("%w: account %v != message %v", ErrSeqnoGap, account.ExtSeqno, message.Seqno)
		return NewExecutionResult().SetError(NewMessageError(err))
	}

	return es.CallVerifyExternal(message, account)
}

func ValidateExternalMessage(es *ExecutionState, message *types.Message) *ExecutionResult {
	check.PanicIfNot(message.IsExternal())

	if message.ChainId != types.DefaultChainId {
		return NewExecutionResult().SetError(NewMessageError(ErrInvalidChainId))
	}

	if account, err := es.GetAccount(message.To); err != nil {
		return NewExecutionResult().SetError(NewMessageError(err))
	} else if account == nil {
		return NewExecutionResult().SetError(NewMessageError(ErrNoPayer))
	}

	switch {
	case message.IsDeploy():
		return validateExternalDeployMessage(es, message)
	case message.IsRefund():
		return NewExecutionResult().SetError(
			NewMessageError(errors.New("refund message is not allowed in external messages")))
	default:
		return validateExternalExecutionMessage(es, message)
	}
}

func NewMessageError(err error) *types.MessageError {
	check.PanicIfNot(err != nil)
	return types.NewMessageError(types.MessageStatusOther, err)
}
