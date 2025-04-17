package execution

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
)

var sharedLogger = logging.NewLogger("execution")

type Payer interface {
	fmt.Stringer
	CanPay(types.Value) bool
	SubBalance(types.Value)
	AddBalance(types.Value) error
}

type dummyPayer struct{}

func NewDummyPayer() dummyPayer {
	return dummyPayer{}
}

func (m dummyPayer) CanPay(types.Value) bool {
	return true
}
func (m dummyPayer) SubBalance(_ types.Value)     {}
func (m dummyPayer) AddBalance(types.Value) error { return nil }
func (m dummyPayer) String() string {
	return "dummy"
}

type transactionPayer struct {
	transaction *types.Transaction
	es          vm.StateDB
}

func NewTransactionPayer(transaction *types.Transaction, es vm.StateDB) Payer {
	// We don't charge system transactions
	if transaction.IsSystem() {
		return dummyPayer{}
	}
	return transactionPayer{
		transaction: transaction,
		es:          es,
	}
}

func (m transactionPayer) CanPay(amount types.Value) bool {
	return true
}

func (m transactionPayer) SubBalance(_ types.Value) {
	// Already paid by sender
}

func (m transactionPayer) AddBalance(delta types.Value) error {
	if m.transaction.RefundTo.IsEmpty() {
		return types.NewError(types.ErrorRefundAddressIsEmpty)
	}

	if _, err := m.es.AddOutTransaction(m.transaction.To, &types.InternalTransactionPayload{
		Kind:  types.RefundTransactionKind,
		To:    m.transaction.RefundTo,
		Value: delta,
	}, 0); err != nil {
		sharedLogger.Error().
			Err(err).
			Stringer(logging.FieldTransactionHash, m.transaction.Hash()).
			Msg("failed to add refund transaction")
	}
	return nil
}

func (m transactionPayer) String() string {
	return "transaction"
}

func NewAccountPayer(account *AccountState, transaction *types.Transaction) accountPayer {
	return accountPayer{
		account:     account,
		transaction: transaction,
	}
}

type accountPayer struct {
	account     *AccountState
	transaction *types.Transaction
}

func (a accountPayer) CanPay(amount types.Value) bool {
	value, overflow := a.transaction.Value.AddOverflow(amount)
	check.PanicIfNot(!overflow)
	return a.account.Balance.Cmp(value) >= 0
}

func (a accountPayer) SubBalance(amount types.Value) {
	check.PanicIfErr(a.account.SubBalance(amount, tracing.BalanceDecreaseGasBuy))
}

func (a accountPayer) AddBalance(amount types.Value) error {
	if err := a.account.AddBalance(amount, tracing.BalanceIncreaseGasReturn); err != nil {
		return types.KeepOrWrapError(types.ErrorInsufficientBalance, err)
	}
	return nil
}

func (a accountPayer) String() string {
	return fmt.Sprintf("account %v", a.transaction.From.Hex())
}

func buyGas(payer Payer, transaction *types.Transaction) error {
	if !payer.CanPay(transaction.FeeCredit) {
		return types.NewWrapError(
			types.ErrorInsufficientFunds,
			fmt.Errorf("%s can't pay %s", payer, transaction.FeeCredit))
	}
	payer.SubBalance(transaction.FeeCredit)
	return nil
}

func refundGas(payer Payer, gasRemaining types.Value) error {
	if gasRemaining.IsZero() {
		return nil
	}
	// Return token for remaining gas, exchanged at the original rate.
	return payer.AddBalance(gasRemaining)
}

func ValidateDeployTransaction(transaction *types.Transaction) types.ExecError {
	deployPayload := types.ParseDeployPayload(transaction.Data)
	if deployPayload == nil {
		return types.NewError(types.ErrorInvalidPayload)
	}

	shardId := transaction.To.ShardId()
	if shardId.IsMainShard() {
		return types.NewError(types.ErrorDeployToMainShard)
	}

	if transaction.To != types.CreateAddress(shardId, *deployPayload) {
		return types.NewError(types.ErrorIncorrectDeploymentAddress)
	}

	return nil
}

func validateExternalDeployTransaction(es *ExecutionState, transaction *types.Transaction) *ExecutionResult {
	check.PanicIfNot(transaction.IsDeploy())

	if err := ValidateDeployTransaction(transaction); err != nil {
		return NewExecutionResult().SetError(err)
	}

	if exists, err := es.ContractExists(transaction.To); err != nil {
		return NewExecutionResult().SetFatal(err)
	} else if exists {
		return NewExecutionResult().SetError(types.NewError(types.ErrorContractAlreadyExists))
	}

	return NewExecutionResult()
}

func validateExternalExecutionTransaction(es *ExecutionState, transaction *types.Transaction) *ExecutionResult {
	check.PanicIfNot(transaction.IsExecution())

	to := transaction.To
	if exists, err := es.ContractExists(to); err != nil {
		return NewExecutionResult().SetFatal(err)
	} else if !exists {
		if len(transaction.Data) > 0 && transaction.Value.IsZero() {
			return NewExecutionResult().SetError(types.NewError(types.ErrorContractDoesNotExist))
		}
		return NewExecutionResult() // send value
	}

	account, err := es.GetAccount(to)
	check.PanicIfErr(err)
	if account.ExtSeqno != transaction.Seqno {
		err = fmt.Errorf("account %v != transaction %v", account.ExtSeqno, transaction.Seqno)
		return NewExecutionResult().SetError(types.NewWrapError(types.ErrorSeqnoGap, err))
	}

	return es.CallVerifyExternal(transaction, account)
}

func ValidateExternalTransaction(es *ExecutionState, transaction *types.Transaction) *ExecutionResult {
	check.PanicIfNot(transaction.IsExternal())

	if transaction.ChainId != types.DefaultChainId {
		return NewExecutionResult().SetError(types.NewError(types.ErrorInvalidChainId))
	}

	if transaction.MaxFeePerGas.IsZero() {
		es.logger.Error().Msg("MaxFeePerGas is zero")
		return NewExecutionResult().SetError(types.NewError(types.ErrorMaxFeePerGasIsZero))
	}

	if account, err := es.GetAccount(transaction.To); err != nil {
		return NewExecutionResult().SetError(types.KeepOrWrapError(types.ErrorNoAccount, err))
	} else if account == nil {
		return NewExecutionResult().SetError(types.NewError(types.ErrorDestinationContractDoesNotExist))
	}

	switch {
	case transaction.IsDeploy():
		return validateExternalDeployTransaction(es, transaction)
	case transaction.IsRefund():
		return NewExecutionResult().SetError(types.NewError(
			types.ErrorRefundTransactionIsNotAllowedInExternalTransactions))
	default:
		return validateExternalExecutionTransaction(es, transaction)
	}
}
