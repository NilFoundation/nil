package commands

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type aggregateFRICmd struct {
	cmdCommon
}

func NewAggregateFRICmd(config CommandConfig) Command {
	return &aggregateFRICmd{
		cmdCommon: makeCmdCommon(config),
	}
}

func (cmd *aggregateFRICmd) MakeCommandDefinition(task *types.Task) (*CommandDefinition, error) {
	binary := cmd.binary
	stage := []string{"--stage", "aggregated-FRI"}
	assignmentTableFile, err := aggregateCircuitDependencies(task, types.PartialProve, types.AssignmentTableDescription)
	if err != nil {
		return nil, err
	}
	assignmentTable := []string{"--assignment-description-file", assignmentTableFile[0]}

	aggChallengeFile, err := findDependencyResult(task, types.AggregatedChallenge, types.AggregatedChallenges)
	if err != nil {
		return nil, err
	}
	aggregatedChallenge := []string{"--aggregated-challenge-file", aggChallengeFile}

	combinedQFiles, err := aggregateCircuitDependencies(task, types.CombinedQ, types.CombinedQPolynomial)
	if err != nil {
		return nil, err
	}
	combinedQ := append([]string{"--input-combined-Q-polynomial-files"}, combinedQFiles...)

	resFiles := make(types.TaskOutputArtifacts)
	filePostfix := fmt.Sprintf(".%v.%v", task.ShardId, task.BlockHash.String())
	resFiles[types.AggregatedFRIProof] = filepath.Join(cmd.outDir, "aggregated_FRI_proof"+filePostfix)
	resFiles[types.ProofOfWork] = filepath.Join(cmd.outDir, "POW"+filePostfix)
	resFiles[types.ConsistencyCheckChallenges] = filepath.Join(cmd.outDir, "challenges"+filePostfix)

	aggFRI := []string{"--proof", resFiles[types.AggregatedFRIProof]}
	POW := []string{"--proof-of-work-file", resFiles[types.ProofOfWork]}
	consistencyChallenges := []string{"--consistency-checks-challenges-file", resFiles[types.ConsistencyCheckChallenges]}
	allArgs := slices.Concat(stage, assignmentTable, aggregatedChallenge, combinedQ, aggFRI, POW, consistencyChallenges)
	execCmd := exec.Command(binary, allArgs...)
	return &CommandDefinition{ExecCommands: []*exec.Cmd{execCmd}, ExpectedResult: resFiles}, execCmd.Err
}
