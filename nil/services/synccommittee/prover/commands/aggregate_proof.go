package commands

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type aggregateProofCmd struct {
	cmdCommon
}

func NewAggregateProofCmd(config CommandConfig) Command {
	return &aggregateProofCmd{
		cmdCommon: makeCmdCommon(config),
	}
}

var _ AfterCommandExecuted = new(aggregateProofCmd)

func collectFinalProofsToAggregate(_ *types.Task) ([]string, error) {
	// TODO: clarify source of proof files
	return nil, nil
}

func (cmd *aggregateProofCmd) MakeCommandDefinition(task *types.Task) (*CommandDefinition, error) {
	binary := "echo" // TODO: enable aggregate proof command once it will be implemented
	stage := []string{"--stage", "aggregate-proofs"}
	blockProofFiles, err := collectFinalProofsToAggregate(task)
	if err != nil {
		return nil, err
	}
	blockProofs := append([]string{"--block-proof"}, blockProofFiles...)

	outFile := filepath.Join(cmd.outDir,
		fmt.Sprintf("aggregated-proof.%v", task.BatchId))
	outArg := []string{"--proof", outFile}

	allArgs := slices.Concat(stage, blockProofs, outArg)
	execCmd := exec.Command(binary, allArgs...)
	return &CommandDefinition{
		ExecCommands:   []*exec.Cmd{execCmd},
		ExpectedResult: types.TaskOutputArtifacts{types.AggregatedProof: outFile},
	}, execCmd.Err
}

func (cmd *aggregateProofCmd) AfterCommandExecuted(
	task *types.Task,
	results types.TaskOutputArtifacts,
) (types.TaskResultData, error) {
	// TODO: pass aggregated proof here
	return types.TaskResultData{}, nil
}
