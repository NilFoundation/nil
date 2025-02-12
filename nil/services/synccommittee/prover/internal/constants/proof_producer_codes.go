package constants

import (
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

// ProofProducerResultCode represents the result codes for proof-producer binary.
// Correspond to the values defined here:
// https://github.com/NilFoundation/placeholder/blob/master/proof-producer/bin/proof-producer/include/nil/proof-generator/command_step.hpp
type ProofProducerResultCode int

const (
	ProofProducerSuccess      ProofProducerResultCode = 0
	ProofProducerIOError      ProofProducerResultCode = 10
	ProofProducerInvalidInput ProofProducerResultCode = 20
	ProofProducerProverError  ProofProducerResultCode = 30
	ProofProducerOutOfMemory  ProofProducerResultCode = 40
	ProofProducerUnknownError ProofProducerResultCode = 0xFF
)

var ProofProducerErrors = map[ProofProducerResultCode]types.TaskErrType{
	ProofProducerIOError:      types.TaskErrIO,
	ProofProducerInvalidInput: types.TaskErrInvalidInputData,
	ProofProducerProverError:  types.TaskErrProofGenerationFailed,
	ProofProducerOutOfMemory:  types.TaskErrOutOfMemory,
	ProofProducerUnknownError: types.TaskErrUnknown,
}
