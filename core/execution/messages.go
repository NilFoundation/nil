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
	gas     types.Gas
	es      *ExecutionState
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

	if err := m.es.AddOutInternal(m.message.To, &types.InternalMessagePayload{
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

func buyGas(payer payer, message *types.Message) error {
	if !payer.CanPay(message.FeeCredit) {
		return fmt.Errorf("%w: %s can't pay %s", ErrInsufficientFunds, payer, message.FeeCredit)
	}
	payer.SubBalance(message.FeeCredit)
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
	if types.IsMainShard(shardId) && message.From != types.MainWalletAddress {
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

func validateExternalExecutionMessage(es *ExecutionState, message *types.Message) (types.Gas, error) {
	check.PanicIfNot(message.IsExecution())

	to := message.To
	if exists, err := es.ContractExists(to); err != nil {
		return 0, err
	} else if !exists {
		if len(message.Data) > 0 && message.Value.IsZero() {
			return 0, ErrContractDoesNotExist
		}
		return 0, nil // Just send value
	}

	account, err := es.GetAccount(to)
	check.PanicIfErr(err)
	if account.ExtSeqno != message.Seqno {
		return 0, fmt.Errorf("%w: account %v != message %v", ErrSeqnoGap, account.ExtSeqno, message.Seqno)
	}

	return es.CallVerifyExternal(message, account)
}

func ValidateExternalMessage(es *ExecutionState, message *types.Message) (types.Gas, error) {
	check.PanicIfNot(message.IsExternal())

	if message.ChainId != types.DefaultChainId {
		return 0, ErrInvalidChainId
	}

	if account, err := es.GetAccount(message.To); err != nil {
		return 0, err
	} else if account == nil {
		return 0, ErrNoPayer
	}

	switch {
	case message.IsDeploy():
		return 0, validateExternalDeployMessage(es, message)
	case message.IsRefund():
		return 0, errors.New("refund message is not allowed in external messages")
	default:
		return validateExternalExecutionMessage(es, message)
	}
}
