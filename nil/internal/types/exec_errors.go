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
	ErrorOutOfGas
	ErrorBounce
	ErrorBuyGas
	ErrorValidation
	ErrorInsufficientBalance
	ErrorNoAccount
	ErrorCodeStoreOutOfGas
	ErrorCallDepthExceeded
	ErrorContractAddressCollision
	ErrorExecutionReverted
	ErrorMaxCodeSizeExceeded
	ErrorMaxInitCodeSizeExceeded
	ErrorInvalidJump
	ErrorWriteProtection
	ErrorReturnDataOutOfBounds
	ErrorGasUintOverflow
	ErrorInvalidCode
	ErrorNonceUintOverflow
	ErrorInvalidInputLength
	ErrorCrossShardMessage
	ErrorStopToken
	ErrorForwardingFailed
	ErrorMessageToMainShard
	ErrorExternalVerificationFailed
	ErrorInvalidMessage
	ErrorInvalidMessageInputUnmarshalFailed
	ErrorOnlyResponseCheckFailed
	ErrorUnexpectedPrecompileType
	ErrorStackUnderflow
	ErrorStackOverflow
	ErrorInvalidOpcode

	// ErrorInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrorInsufficientFunds

	// ErrorGasUint64Overflow is returned when calculating gas usage.
	ErrorGasUint64Overflow

	// ErrorInternalMessageValidationFailed is returned when no corresponding outgoing message is found.
	ErrorInternalMessageValidationFailed

	// ErrorNoAccountToPayFees is returned when no account at address specified to pay fees.
	ErrorNoAccountToPayFees

	// ErrorContractAlreadyExists is returned when attempt to deploy code to address of already deployed contract.
	ErrorContractAlreadyExists

	// ErrorContractDoesNotExist is returned when attempt to call non-existent contract.
	ErrorContractDoesNotExist

	// ErrorSeqnoGap is returned when message seqno does not match the seqno of the recipient.
	ErrorSeqnoGap

	// ErrorExternalMsgVerificationFailed is returned when verifyExternal call fails.
	ErrorExternalMsgVerificationFailed

	// ErrorInvalidChainId is returned when message chain id is different from DefaultChainId.
	ErrorInvalidChainId

	// ErrorInvalidPayload is returned when message payload is invalid (e.g., less than 32 bytes).
	ErrorInvalidPayload

	// ErrorDeployToMainShard is returned when a non-system wallet requests deploy to the main shard.
	ErrorDeployToMainShard
	ErrorShardIdIsTooBig
	ErrorAbiPackFailed
	ErrorAbiUnpackFailed

	ErrorIncorrectDeploymentAddress
	ErrorMessageFeeForwardingFailed
	ErrorRefundMessageIsNotAllowedInExternalMessages
	ErrorPrecompileTooShortCallData
	ErrorPrecompileWrongNumberOfArguments
	ErrorPrecompileInvalidCurrencyArray
	ErrorPrecompileStateDbReturnedError
	ErrorOnlyMainShardContractsCanChangeConfig
	ErrorPrecompileConfigSetParamFailed
	ErrorPrecompileConfigGetParamFailed
	ErrorAwaitCallCalledFromNotTopLevel
	ErrorAwaitCallTooLowResponseProcessingGas
	ErrorAwaitCallTooShortContextData
	ErrorAsyncDeployMustNotHaveCurrency
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
	msg string
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
	msg string
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

func NewVerboseError(code ErrorCode, msg string) ExecError {
	return &VerboseError{BaseError{code}, msg}
}

func NewVmVerboseError(code ErrorCode, msg string) ExecError {
	return &VmVerboseError{VmError{BaseError{code}}, msg}
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
	return e.BaseError.Error() + ": " + e.msg
}

func (e VerboseError) Unwrap() error {
	return &e.BaseError
}

func (e VmVerboseError) Error() string {
	return e.VmError.Error() + ": " + e.msg
}

func (e VmVerboseError) Unwrap() error {
	return &e.VmError
}
