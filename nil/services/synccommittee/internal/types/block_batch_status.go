package types

type BatchStatus int8

const (
	_ BatchStatus = iota

	// BatchStatusCreated indicates that the batch has been created and currently is empty.
	BatchStatusCreated

	// BatchStatusPending indicates that the batch is in a pending state, awaiting to be filled and sealed.
	BatchStatusPending

	// BatchStatusSealed indicates that the batch has been sealed and is no longer modifiable.
	BatchStatusSealed

	// BatchStatusCommitted indicates that the batch has been successfully committed to L1.
	BatchStatusCommitted

	// BatchStatusProofTaskCreated indicates that a proof task for the batch has been successfully created.
	BatchStatusProofTaskCreated

	// BatchStatusProved indicates that the corresponding proof task has been successfully completed.
	BatchStatusProved

	// BatchStatusUpdatedState indicates that the corresponding state update has been successfully submitted to L1.
	BatchStatusUpdatedState
)

func (b BatchStatus) IsSealed() bool {
	return b != BatchStatusCreated && b != BatchStatusPending
}
