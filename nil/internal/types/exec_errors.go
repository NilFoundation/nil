package types

import (
	"errors"

	"github.com/NilFoundation/nil/nil/common/check"
)

// This file contains an implementation of errors handling for the execution phase. Each error is uniquely identified by
// an integer number (ErrorCode), which is then saved in the transaction Receipt.
//
// There are two main reasons to use this approach to errors handling:
// 1. Ease of adding new errors. To do this, just add a new `ErrorCode` enum constant and use it like this:
//    `types.NewError(types.ErrorSomeNewError)`. The name of the constant is also a string representation of the error,
//    e.g. `ErrorOutOfGas.String() => "OutOfGas"`.
// 2. More accurate identification of errors in receipts. Since it is easy to add new error, we can add as much error
//    codes as we wish. For any particular error case, we can add a dedicated error code. As a result, it should help to
//    understand the reason of the failed transaction through its receipt.

type ErrorCode uint32

const (
	ErrorSuccess ErrorCode = iota
	ErrorUnknown
	ErrorExecution

	// ErrorOutOfGasStart is auxiliary code, should never be returned
	ErrorOutOfGasStart
	// ErrorOutOfGas is general out of gas error
	ErrorOutOfGas
	// ErrorOutOfGasDynamic is out of gas error happened during charge of dynamic gas
	ErrorOutOfGasDynamic
	// ErrorOutOfGasForPrecompile is out of gas error happened during precompile contract execution
	ErrorOutOfGasForPrecompile
	// ErrorOutOfGasStorage is returned if there are no funds for paying code storage fee during deployment
	ErrorOutOfGasStorage
	// ErrorOutOfGasEnd is auxiliary code, should never be returned
	ErrorOutOfGasEnd
	// ErrorBuyGas is returned when purchasing sufficient gas is not possible for some reason
	ErrorBuyGas
	// ErrorValidation is returned if the transaction is invalid
	ErrorValidation
	// ErrorInsufficientBalance is returned if the account does not have enough money to pay for a some operation
	ErrorInsufficientBalance
	// ErrorNoAccount is returned if the account does not exist
	ErrorNoAccount
	// ErrorCallDepthExceeded is returned if the sync call depth is exceeded
	ErrorCallDepthExceeded
	// ErrorContractAddressCollision is returned if the contract address is already in use
	ErrorContractAddressCollision
	// ErrorExecutionReverted is returned if the EVM execution was reverted
	ErrorExecutionReverted
	// ErrorMaxCodeSizeExceeded is returned if the code size is too big (EIP-158)
	ErrorMaxCodeSizeExceeded
	// ErrorMaxInitCodeSizeExceeded is returned if the init code size is too big
	ErrorMaxInitCodeSizeExceeded
	// ErrorInvalidJump is returned if during EVM execution the jump destination is invalid
	ErrorInvalidJump
	// ErrorWriteProtection is returned if the contract tries to change state during a read-only execution
	ErrorWriteProtection
	// ErrorReturnDataOutOfBounds is returned if the contract tries to return data outside the valid bounds
	ErrorReturnDataOutOfBounds
	// ErrorGasUintOverflow is returned if the gas overflows uint64
	ErrorGasUintOverflow
	// ErrorInvalidCode is returned if the code is started with 0xEF (EIP-3541)
	ErrorInvalidCode
	// ErrorNonceUintOverflow is returned if the nonce value overflows uint64
	ErrorNonceUintOverflow
	// ErrorCrossShardTransaction is returned if the sync operation is performed between shards
	ErrorCrossShardTransaction
	// ErrorStopToken is an internal error, should never be returned
	ErrorStopToken
	// ErrorForwardingFailed is returned if the message forwarding failed
	ErrorForwardingFailed
	// ErrorTransactionToMainShard is returned if the transaction tries to make an async call to the main shard
	ErrorTransactionToMainShard
	// ErrorExternalVerificationFailed is returned if verification of the external failed
	ErrorExternalVerificationFailed
	// ErrorInvalidTransactionInputUnmarshalFailed is returned from SendRaw precompile if the given transaction cannot
	// be unmarshal
	ErrorInvalidTransactionInputUnmarshalFailed
	// ErrorOnlyResponseCheckFailed is returned from `onlyResponse` precompile if inbound transaction is not a response
	ErrorOnlyResponseCheckFailed
	// ErrorUnexpectedPrecompileType is returned if the precompile type is invalid
	ErrorUnexpectedPrecompileType
	// ErrorStackUnderflow is returned if the EVM stack underflows
	ErrorStackUnderflow
	// ErrorStackOverflow is returned if the EVM stack overflows
	ErrorStackOverflow
	// ErrorInvalidOpcode is returned if the EVM execution encounters an invalid opcode
	ErrorInvalidOpcode

	// ErrorInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrorInsufficientFunds

	// ErrorGasUint64Overflow is returned when calculating gas usage.
	ErrorGasUint64Overflow

	// ErrorInternalTransactionValidationFailed is returned when no corresponding outgoing transaction is found.
	ErrorInternalTransactionValidationFailed

	// ErrorDestinationContractDoesNotExist is returned when no account exists and the destination address.
	// If you encounter this error, you probably forgot to top-up the address before deploying.
	ErrorDestinationContractDoesNotExist

	// ErrorContractAlreadyExists is returned when attempt to deploy code to address of already deployed contract.
	ErrorContractAlreadyExists

	// ErrorContractDoesNotExist is returned when attempt to call non-existent contract.
	ErrorContractDoesNotExist

	// ErrorSeqnoGap is returned when transaction seqno does not match the seqno of the recipient.
	ErrorSeqnoGap

	// ErrorExternalMsgVerificationFailed is returned when verifyExternal call fails.
	ErrorExternalMsgVerificationFailed

	// ErrorInvalidChainId is returned when transaction chain id is different from DefaultChainId.
	ErrorInvalidChainId

	// ErrorInvalidPayload is returned when transaction payload is invalid (e.g., less than 32 bytes).
	ErrorInvalidPayload

	// ErrorDeployToMainShard is returned when a non-system smart account requests deploy to the main shard.
	ErrorDeployToMainShard
	// ErrorShardIdIsTooBig is returned when the specified shard id is greater than available shards.
	ErrorShardIdIsTooBig
	// ErrorAbiPackFailed is returned when some precompile fails to pack the ABI.
	ErrorAbiPackFailed
	// ErrorAbiUnpackFailed is returned when some precompile fails to unpack the ABI.
	ErrorAbiUnpackFailed

	// ErrorIncorrectDeploymentAddress is returned when trying to deploy contract to address which is not equal to one
	// calculated from deployed code and salt.
	ErrorIncorrectDeploymentAddress
	// ErrorRefundTransactionIsNotAllowedInExternalTransactions is returned when the external transaction contains a
	// refund flag
	ErrorRefundTransactionIsNotAllowedInExternalTransactions
	// ErrorPrecompileTooShortCallData is returned when the call data is too short for the precompile
	ErrorPrecompileTooShortCallData
	// ErrorPrecompileWrongNumberOfArguments is returned when the number of arguments is incorrect for the precompile
	ErrorPrecompileWrongNumberOfArguments
	// ErrorPrecompileInvalidTokenArray is returned when the token array argument, passed to the precompile, is invalid
	ErrorPrecompileInvalidTokenArray
	// ErrorPrecompileTokenArrayIsTooBig is returned when the token array size is greater than 256
	ErrorPrecompileTokenArrayIsTooBig
	// ErrorPrecompileStateDbReturnedError is an internal error indicating that the Execution returned an error
	ErrorPrecompileStateDbReturnedError
	// ErrorOnlyMainShardContractsCanChangeConfig is returned when a contract from a shard other than the main one tries
	// to change on-chain config
	ErrorOnlyMainShardContractsCanChangeConfig
	// ErrorPrecompileWrongCaller is returned when the caller of the precompile is not the expected one
	ErrorPrecompileWrongCaller
	// ErrorPrecompileWrongVersion is returned when the version of the precompile is not the expected one
	ErrorPrecompileWrongVersion
	// ErrorPrecompileBadArgument is returned when the precompile receives an invalid argument
	ErrorPrecompileBadArgument
	// ErrorPrecompileConfigSetParamFailed is returned when the precompile fails to set the config parameter
	ErrorPrecompileConfigSetParamFailed
	// ErrorPrecompileConfigGetParamFailed is returned when the precompile fails to get the config parameter
	ErrorPrecompileConfigGetParamFailed
	// ErrorAwaitCallCalledFromNotTopLevel is returned when the await call is called from a contract which was
	// synchronously called from another contract.
	ErrorAwaitCallCalledFromNotTopLevel
	// ErrorAwaitCallTooLowResponseProcessingGas is returned when the response processing gas is too low for the await
	// call.
	ErrorAwaitCallTooLowResponseProcessingGas
	// ErrorAwaitCallTooShortContextData is returned when the context data is too short for the await call.
	ErrorAwaitCallTooShortContextData
	// ErrorAsyncDeployMustNotHaveToken is returned when the async deploy transaction contains custom token. It is not
	// allowed to transfer custom tokens within async deploy transaction.
	ErrorAsyncDeployMustNotHaveToken

	// ErrorEmitLogFailed is returned when the execution state fails to add a log. Probably the limit of logs is
	// reached.
	ErrorEmitLogFailed
	// ErrorEmitDebugLogFailed is returned when the execution state fails to add a debug log. Probably the limit of logs
	// is reached.
	ErrorEmitDebugLogFailed
	// ErrorRefundAddressIsEmpty is returned when the transaction contains an empty refund address during gas refund.
	ErrorRefundAddressIsEmpty
	// ErrorGasRefundFailed is a general error for failed gas refund.
	ErrorGasRefundFailed
	// ErrorPanicDuringExecution is returned when a panic occurs during the execution of the transaction.
	ErrorPanicDuringExecution
	// ErrorBaseFeeTooHigh is returned when the base fee is higher than MaxFeePerGas specified in the message.
	ErrorBaseFeeTooHigh
	// ErrorMaxFeePerGasIsZero is returned when the MaxFeePerGas is zero. It is not allowed to have zero MaxFeePerGas.
	ErrorMaxFeePerGasIsZero
)

type ExecError interface {
	error
	Code() ErrorCode
}

var _ ExecError = new(BaseError)

type BaseError struct {
	code ErrorCode
}

type VerboseError struct {
	BaseError
	txn string
}

type WrapError struct {
	BaseError
	inner error
}

type VmError struct {
	BaseError
}

type VmVerboseError struct {
	VmError
	txn string
}

func NewError(code ErrorCode) ExecError {
	return &BaseError{code}
}

func IsValidError(err error) bool {
	return ToError(err) != nil
}

func ToBaseError(err error) *BaseError {
	var base *BaseError
	if errors.As(err, &base) {
		return base
	}
	return nil
}

func ToError(err error) ExecError {
	if e, ok := err.(ExecError); ok { //nolint:errorlint
		return e
	}
	return nil
}

func IsVmError(err error) bool {
	var e *VmError
	return errors.As(err, &e)
}

func IsOutOfGasError(err error) bool {
	if !IsValidError(err) {
		return false
	}
	return GetErrorCode(err) >= ErrorOutOfGasStart && GetErrorCode(err) <= ErrorOutOfGasEnd
}

func GetErrorCode(err error) ErrorCode {
	if base := ToBaseError(err); base != nil {
		return base.Code()
	}
	return ErrorUnknown
}

func NewVmError(code ErrorCode) ExecError {
	return &VmError{BaseError{code}}
}

func NewWrapError(code ErrorCode, err error) ExecError {
	// Nested errors(Error type) are not allowed because error code must be unique.
	check.PanicIfNotf(!IsValidError(err), "nested errors are prohibited")
	return &WrapError{BaseError{code}, err}
}

func KeepOrWrapError(code ErrorCode, err error) ExecError {
	if e := ToError(err); e != nil {
		return e
	}
	return NewWrapError(code, err)
}

func NewVerboseError(code ErrorCode, txn string) ExecError {
	return &VerboseError{BaseError{code}, txn}
}

func NewVmVerboseError(code ErrorCode, txn string) ExecError {
	return &VmVerboseError{VmError{BaseError{code}}, txn}
}

func (e BaseError) Error() string {
	return e.Code().String()
}

func (e BaseError) Code() ErrorCode {
	return e.code
}

func (e VmError) Unwrap() error {
	return &e.BaseError
}

func (e WrapError) Error() string {
	return e.BaseError.Error() + ": " + e.inner.Error()
}

func (e WrapError) Unwrap() error {
	return e.inner
}

func (e VerboseError) Error() string {
	return e.BaseError.Error() + ": " + e.txn
}

func (e VerboseError) Unwrap() error {
	return &e.BaseError
}

func (e VmVerboseError) Error() string {
	return e.VmError.Error() + ": " + e.txn
}

func (e VmVerboseError) Unwrap() error {
	return &e.VmError
}

//go:generate stringer -type=ErrorCode -trimprefix=Error
