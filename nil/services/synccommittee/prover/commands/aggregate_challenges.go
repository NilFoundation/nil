package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type aggregateChallengesCmd struct {
	cmdCommon
}

func NewAggregateChallengesCmd(config CommandConfig) Command {
	return &aggregateChallengesCmd{
		cmdCommon: makeCmdCommon(config),
	}
}

var _ BeforeCommandExecuted = new(aggregateChallengesCmd)

func (cmd *aggregateChallengesCmd) BeforeCommandExecuted(
	ctx context.Context,
	task *types.Task,
	results types.TaskOutputArtifacts,
) error {
	// Collect values from theta files
	thetaPowerFiles, err := aggregateCircuitDependencies(task, types.PartialProve, types.ThetaPower)
	if err != nil {
		return err
	}
	thetas := make([]int, types.CircuitAmount)
	for i, filename := range thetaPowerFiles {
		bytes, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("Cannot read theta power file: %w", err)
		}
		num, err := strconv.Atoi(strings.TrimSpace(string(bytes)))
		if err != nil {
			return fmt.Errorf("Couldn't convert theta file content to integer: %w", err)
		}
		thetas[i] = num
	}

	// Compute partial sums that would be actual arguments for combined Q stage handling
	thetaPartialSums := make([]int, types.CircuitAmount)
	for i := range types.CircuitAmount - 1 {
		thetaPartialSums[i+1] = thetas[i] + thetaPartialSums[i]
	}

	// Write partial sums into file
	bytes, err := json.Marshal(thetaPartialSums)
	if err != nil {
		return err
	}
	aggregateFileName := filepath.Join(cmd.outDir, fmt.Sprintf("aggregated_thetas.%v", task.BatchId))
	results[types.AggregatedThetaPowers] = aggregateFileName
	return os.WriteFile(aggregateFileName, bytes, 0o644) //nolint:gosec
}

func (cmd *aggregateChallengesCmd) MakeCommandDefinition(task *types.Task) (*CommandDefinition, error) {
	binary := cmd.binary
	stage := []string{"--stage", "generate-aggregated-challenge"}
	inputFiles, err := aggregateCircuitDependencies(task, types.PartialProve, types.PartialProofChallenges)
	if err != nil {
		return nil, err
	}
	inputs := append([]string{"--input-challenge-files"}, inputFiles...)
	outFile := filepath.Join(cmd.outDir,
		fmt.Sprintf("aggregated_challenges.%v", task.BatchId))
	outArg := []string{"--aggregated-challenge-file", outFile}
	allArgs := slices.Concat(stage, inputs, outArg)
	execCmd := exec.Command(binary, allArgs...)
	return &CommandDefinition{
		ExecCommands:   []*exec.Cmd{execCmd},
		ExpectedResult: types.TaskOutputArtifacts{types.AggregatedChallenges: outFile},
	}, execCmd.Err
}
