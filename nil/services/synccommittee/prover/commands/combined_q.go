package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type combinedQCmd struct {
	cmdCommon
}

func NewCombinedQCmd(config CommandConfig) Command {
	return &combinedQCmd{
		cmdCommon: makeCmdCommon(config),
	}
}

func fetchStartingThetaPower(task *types.Task) (int, error) {
	aggThetasFile, err := findDependencyResult(task, types.AggregatedChallenge, types.AggregatedThetaPowers)
	if err != nil {
		return 0, err
	}
	var aggregatedThetas []int
	marshaledThetas, err := os.ReadFile(aggThetasFile)
	if err != nil {
		return 0, fmt.Errorf("Unable to read aggregated theta file: %w", err)
	}
	if err := json.Unmarshal(marshaledThetas, &aggregatedThetas); err != nil {
		return 0, fmt.Errorf("Unable to unmarshal aggregated theta file: %w", err)
	}
	return aggregatedThetas[uint8(task.CircuitType)-types.CircuitStartIndex], nil
}

func (cmd *combinedQCmd) MakeCommandDefinition(task *types.Task) (*CommandDefinition, error) {
	binary := cmd.binary
	stage := []string{"--stage", "compute-combined-Q"}
	commitmentStateFile, err := findDependencyResult(task, types.PartialProve, types.CommitmentState)
	if err != nil {
		return nil, err
	}
	commitmentState := []string{"--commitment-state-file", commitmentStateFile}

	aggChallengesFile, err := findDependencyResult(task, types.AggregatedChallenge, types.AggregatedChallenges)
	if err != nil {
		return nil, err
	}
	aggregateChallenges := []string{"--aggregated-challenge-file", aggChallengesFile}

	// Fetch starting theta power from file
	startingPower, err := fetchStartingThetaPower(task)
	if err != nil {
		return nil, err
	}
	startingPowerArg := []string{"--combined-Q-starting-power", strconv.Itoa(startingPower)}
	outFile := filepath.Join(cmd.outDir,
		fmt.Sprintf("combined_Q.%v.%v", circuitIdx(task.CircuitType), task.BatchId))
	outArg := []string{"--combined-Q-polynomial-file", outFile}

	allArgs := slices.Concat(stage, commitmentState, aggregateChallenges, startingPowerArg, outArg)
	execCmd := exec.Command(binary, allArgs...)
	return &CommandDefinition{
		ExecCommands:   []*exec.Cmd{execCmd},
		ExpectedResult: types.TaskOutputArtifacts{types.CombinedQPolynomial: outFile},
	}, execCmd.Err
}
