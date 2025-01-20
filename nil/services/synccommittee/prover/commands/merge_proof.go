package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type mergeProofCmd struct {
	cmdCommon
}

func NewMergeProofCmd(config CommandConfig) Command {
	return &mergeProofCmd{
		cmdCommon: makeCmdCommon(config),
	}
}

var _ AfterCommandExecuted = new(mergeProofCmd)

func (cmd *mergeProofCmd) MakeCommandDefinition(task *types.Task) (*CommandDefinition, error) {
	binary := cmd.binary
	stage := []string{"--stage", "merge-proofs"}
	partialProofFiles, err := aggregateCircuitDependencies(task, types.PartialProve, types.PartialProof)
	if err != nil {
		return nil, err
	}
	partialProofs := append([]string{"--partial-proof"}, partialProofFiles...)

	LPCCheckFiles, err := aggregateCircuitDependencies(task, types.FRIConsistencyChecks, types.LPCConsistencyCheckProof)
	if err != nil {
		return nil, err
	}
	LPCChecks := append([]string{"--initial-proof"}, LPCCheckFiles...)

	aggFRIFile, err := findDependencyResult(task, types.AggregatedFRI, types.AggregatedFRIProof)
	if err != nil {
		return nil, err
	}
	aggFRI := []string{"--aggregated-FRI-proof", aggFRIFile}

	outFile := filepath.Join(cmd.outDir,
		fmt.Sprintf("final-proof.%v.%v", task.ShardId, task.BlockHash.String()))
	outArg := []string{"--proof", outFile}

	allArgs := slices.Concat(stage, partialProofs, LPCChecks, aggFRI, outArg)
	execCmd := exec.Command(binary, allArgs...)
	return &CommandDefinition{
		ExecCommands:   []*exec.Cmd{execCmd},
		ExpectedResult: types.TaskOutputArtifacts{types.FinalProof: outFile},
	}, execCmd.Err
}

func (*mergeProofCmd) AfterCommandExecuted(task *types.Task, results types.TaskOutputArtifacts) (types.TaskResultData, error) {
	mergedProofFile := results[types.FinalProof]
	proofContent, err := os.ReadFile(mergedProofFile)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch final proof data: %w", err)
	}
	return proofContent, nil
}
