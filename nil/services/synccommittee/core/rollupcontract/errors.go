package rollupcontract

import "errors"

var (
	ErrBatchAlreadyFinalized                     = errors.New("batch already finalized")
	ErrBatchAlreadyCommitted                     = errors.New("batch already committed")
	ErrBatchNotCommitted                         = errors.New("batch has not been committed")
	ErrInvalidBatchIndex                         = errors.New("batch index is invalid")
	ErrInvalidVersionedHash                      = errors.New("versioned hash is invalid")
	ErrInvalidOldStateRoot                       = errors.New("invalid old state root")
	ErrInvalidNewStateRoot                       = errors.New("invalid new state root")
	ErrInvalidValidityProof                      = errors.New("invalid validity proof")
	ErrEmptyDataProofs                           = errors.New("empty data proofs")
	ErrDataProofsAndBlobCountMismatch            = errors.New("data proofs and blob count mismatch")
	ErrNewStateRootAlreadyFinalized              = errors.New("new state root already finalized")
	ErrOldStateRootMismatch                      = errors.New("old state root mismatch")
	ErrIncorrectDataProofSize                    = errors.New("incorrect data proof size")
	ErrL1BridgeMessengerAddressNotSet            = errors.New("l1 bridge messenger address is not set")
	ErrInconsistentDepositNonce                  = errors.New("inconsistent deposit nonce")
	ErrInvalidPublicDataInfo                     = errors.New("invalid public data info")
	ErrL1MessageHashMismatch                     = errors.New("l1 message hash mismatch")
	ErrInvalidDataProofItem                      = errors.New("invalid data proof item")
	ErrInvalidPublicInputForProof                = errors.New("invalid public input for proof")
	ErrCallPointEvaluationPrecompileFailed       = errors.New("call point evaluation precompile failed")
	ErrUnexpectedPointEvaluationPrecompileOutput = errors.New("unexpected point evaluation precompile output")
)
