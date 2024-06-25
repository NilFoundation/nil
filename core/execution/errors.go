package execution

import "errors"

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

	// ErrContractDoesNotExist is returned when attempt to call non-existent contract.
	ErrContractDoesNotExist = errors.New("contract does not exist")

	// ErrSeqnoGap is returned when message seqno does not match the seqno of the recipient.
	ErrSeqnoGap = errors.New("seqno gap")

	// ErrInvalidSignature is returned when verifyExternal call fails.
	ErrInvalidSignature = errors.New("invalid signature")

	// ErrInvalidChainId is returned when message chain id is different from DefaultChainId.
	ErrInvalidChainId = errors.New("invalid chainId")

	// ErrInvalidPayload is returned when message payload is invalid (e.g., less than 32 bytes).
	ErrInvalidPayload = errors.New("invalid payload")

	// ErrDeployToMainShard is returned when a non-system wallet requests deploy to the main shard.
	ErrDeployToMainShard = errors.New("attempt to deploy to main shard from non-system wallet")

	// ErrIncorrectDeploymentAddress is returned when the deployment address does not correspond to the payload.
	ErrIncorrectDeploymentAddress = errors.New("incorrect deployment address")
)
