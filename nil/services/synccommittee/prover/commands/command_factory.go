package commands

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type CommandFactory struct {
	config CommandConfig
	logger zerolog.Logger
}

func NewCommandFactory(config CommandConfig, logger zerolog.Logger) *CommandFactory {
	return &CommandFactory{
		config: config,
		logger: logger,
	}
}

func (factory *CommandFactory) MakeHandlerCommandForTaskType(taskType types.TaskType) (Command, error) {
	switch taskType {
	case types.PartialProve:
		return NewPartialProofCmd(factory.config, factory.logger), nil
	case types.AggregatedChallenge:
		return NewAggregateChallengesCmd(factory.config), nil
	case types.CombinedQ:
		return NewCombinedQCmd(factory.config), nil
	case types.AggregatedFRI:
		return NewAggregateFRICmd(factory.config), nil
	case types.FRIConsistencyChecks:
		return NewConsistencyCheckCmd(factory.config), nil
	case types.MergeProof:
		return NewMergeProofCmd(factory.config), nil
	case types.AggregateProofs:
		return NewAggregateProofCmd(factory.config), nil
	case types.ProofBlock:
		err := errors.New("ProofBlock task type is not supposed to be encountered in prover task handler")
		return nil, err
	case types.TaskTypeNone:
		err := errors.New("Task has unspecified type")
		return nil, err
	default:
		err := fmt.Errorf("Unhandled task type: %v", taskType.String())
		return nil, err
	}
}
