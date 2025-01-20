package commands

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type consistencyCheckCmd struct {
	cmdCommon
}

func NewConsistencyCheckCmd(config CommandConfig) Command {
	return &consistencyCheckCmd{
		cmdCommon: makeCmdCommon(config),
	}
}

func (cmd *consistencyCheckCmd) MakeCommandDefinition(task *types.Task) (*CommandDefinition, error) {
	binary := cmd.binary
	stage := []string{"--stage", "consistency-checks"}
	commitmentStateFile, err := findDependencyResult(task, types.PartialProve, types.CommitmentState)
	if err != nil {
		return nil, err
	}
	commitmentState := []string{"--commitment-state-file", commitmentStateFile}
	combinedQFile, err := findDependencyResult(task, types.CombinedQ, types.CombinedQPolynomial)
	if err != nil {
		return nil, err
	}
	combinedQ := []string{"--combined-Q-polynomial-file", combinedQFile}
	consistencyChallengeFile, err := findDependencyResult(task, types.AggregatedFRI, types.ConsistencyCheckChallenges)
	if err != nil {
		return nil, err
	}
	consistencyChallenges := []string{"--consistency-checks-challenges-file", consistencyChallengeFile}

	outFile := filepath.Join(cmd.outDir,
		fmt.Sprintf("LPC_consistency_check_proof.%v.%v.%v", circuitIdx(task.CircuitType), task.ShardId, task.BlockHash.String()))
	outArg := []string{"--proof", outFile}

	allArgs := slices.Concat(stage, commitmentState, combinedQ, consistencyChallenges, outArg)
	execCmd := exec.Command(binary, allArgs...)
	return &CommandDefinition{
		ExecCommands:   []*exec.Cmd{execCmd},
		ExpectedResult: types.TaskOutputArtifacts{types.LPCConsistencyCheckProof: outFile},
	}, execCmd.Err
}
