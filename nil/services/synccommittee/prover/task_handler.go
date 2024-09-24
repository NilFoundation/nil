package prover

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type taskHandler struct {
	requestHandler api.TaskRequestHandler
	logger         zerolog.Logger
	config         taskHandlerConfig
}

func newTaskHandler(requestHandler api.TaskRequestHandler, logger zerolog.Logger, config taskHandlerConfig) api.TaskHandler {
	return &taskHandler{
		requestHandler: requestHandler,
		logger:         logger,
		config:         config,
	}
}

type taskHandlerConfig struct {
	AssignerBinary      string
	ProofProducerBinary string
	OutDir              string
}

type commandDescription struct {
	runCommand     *exec.Cmd
	expectedResult types.TaskResultFiles
}

func circuitTypeToArg(ct types.CircuitType) string {
	switch ct {
	case types.Bytecode:
		return "bytecode"
	case types.MPT:
		return "mpt"
	case types.ReadWrite:
		return "rw"
	case types.ZKEVM:
		return "zkevm"
	default:
		panic("Unknown circuit type")
	}
}

func circuitIdx(ct types.CircuitType) uint8 {
	switch ct {
	case types.Bytecode:
		return 1
	case types.ReadWrite:
		return 2
	case types.MPT:
		return 3
	case types.ZKEVM:
		return 4
	default:
		panic("Unknown circuit type")
	}
}

func collectDependencyFiles(task *types.Task, taskType types.TaskType, resultType types.ProverResultType) []string {
	depFiles := []string{}
	for _, res := range task.Dependencies {
		if res.Type == taskType {
			path, ok := res.DataAddresses[resultType]
			if !ok {
				panic("Inconsistent task")
			}
			depFiles = append(depFiles, path)
		}
	}
	return depFiles
}

func (h *taskHandler) makePartialProofTaskCommand(task *types.Task) commandDescription {
	binary := h.config.AssignerBinary
	blockData := []string{"--shard-id", task.ShardId.String(), "--block-hash", task.BlockHash.String()}
	outDir := []string{"--path", h.config.OutDir}
	circuit := []string{"--target-circuits", circuitTypeToArg(task.CircuitType)}
	allArgs := slices.Concat(blockData, outDir, circuit)
	cmd := exec.Command(binary, allArgs...)
	resFiles := make(types.TaskResultFiles)
	filePostfix := fmt.Sprintf(".%v.%v.%v", circuitIdx(task.CircuitType), task.ShardId, task.BlockHash.String())
	resFiles[types.PartialProofChallenges] = filepath.Join(h.config.OutDir, "challenge"+filePostfix)
	resFiles[types.AssignmentTableDescription] = filepath.Join(h.config.OutDir, "assignment_table_description"+filePostfix)
	resFiles[types.PartialProof] = filepath.Join(h.config.OutDir, "proof"+filePostfix)
	resFiles[types.CommitmentState] = filepath.Join(h.config.OutDir, "commitment_state"+filePostfix)
	return commandDescription{runCommand: cmd, expectedResult: resFiles}
}

func (h *taskHandler) makeAggregateChallengesTaskCommand(task *types.Task) commandDescription {
	binary := h.config.ProofProducerBinary
	stage := []string{"--stage=\"generate-aggregated-challenge\""}
	inputFiles := collectDependencyFiles(task, types.PartialProve, types.PartialProofChallenges)
	if len(inputFiles) != int(types.CircuitAmount) {
		panic("Insufficient input for task")
	}
	inputs := append([]string{"--input-challenge-files"}, inputFiles...)
	outFile := fmt.Sprintf("aggregated_challenges.%v.%v", task.ShardId, task.BlockHash.String())
	outArg := []string{fmt.Sprintf("--aggregated-challenge-file=\"%v\"", filepath.Join(h.config.OutDir, outFile))}
	allArgs := slices.Concat(stage, inputs, outArg)
	cmd := exec.Command(binary, allArgs...)
	return commandDescription{
		runCommand:     cmd,
		expectedResult: types.TaskResultFiles{types.AggregatedChallenges: outFile},
	}
}

func (h *taskHandler) makeCombinedQCommand(task *types.Task) commandDescription {
	binary := h.config.ProofProducerBinary
	stage := []string{"--stage=\"compute-combined-Q\""}
	commitmentStateFile := collectDependencyFiles(task, types.PartialProve, types.CommitmentState)
	if len(commitmentStateFile) != 1 {
		panic("Insufficient input for task")
	}
	commitmentState := []string{fmt.Sprintf("--commitment-state-file=\"%v\"", commitmentStateFile[0])}

	aggChallengesFile := collectDependencyFiles(task, types.AggregatedChallenge, types.AggregatedChallenges)
	if len(aggChallengesFile) != 1 {
		panic("Insufficient input for task")
	}
	aggregateChallenges := []string{fmt.Sprintf("--aggregated-challenge-file=\"%v\"", aggChallengesFile[0])}

	startingPower := []string{"--combined-Q-starting-power=0"} // TODO: compute it properly from dependencies
	outFile := fmt.Sprintf("combined_Q.%v.%v.%v", circuitIdx(task.CircuitType), task.ShardId, task.BlockHash.String())
	outArg := []string{fmt.Sprintf("--combined-Q-polynomial-file=\"%v\"", filepath.Join(h.config.OutDir, outFile))}

	allArgs := slices.Concat(stage, commitmentState, aggregateChallenges, startingPower, outArg)
	cmd := exec.Command(binary, allArgs...)
	return commandDescription{
		runCommand:     cmd,
		expectedResult: types.TaskResultFiles{types.CombinedQPolynomial: outFile},
	}
}

func (h *taskHandler) makeAggregateFRICommand(task *types.Task) commandDescription {
	binary := h.config.ProofProducerBinary
	stage := []string{"--stage=\"aggregated-FRI\""}
	assignmentTableFile := collectDependencyFiles(task, types.PartialProve, types.AssignmentTableDescription)
	if len(assignmentTableFile) != 1 {
		panic("Insufficient input for task")
	}
	assignmentTable := []string{fmt.Sprintf("--assignment-description-file=\"%v\"", assignmentTableFile[0])}

	aggChallengeFile := collectDependencyFiles(task, types.AggregatedChallenge, types.AggregatedChallenges)
	if len(aggChallengeFile) != 1 {
		panic("Insufficient input for task")
	}
	aggregatedChallenge := []string{fmt.Sprintf("--aggregated-challenge-file=\"%v\"", aggChallengeFile[0])}

	combinedQFiles := collectDependencyFiles(task, types.CombinedQ, types.CombinedQPolynomial)
	if len(combinedQFiles) != int(types.CircuitAmount) {
		panic("Insufficient input for task")
	}
	combinedQ := append([]string{"--input-combined-Q-polynomial-files"}, combinedQFiles...)

	resFiles := make(types.TaskResultFiles)
	filePostfix := fmt.Sprintf(".%v.%v", task.ShardId, task.BlockHash.String())
	resFiles[types.AggregatedFRIProof] = filepath.Join(h.config.OutDir, "aggregated_FRI_proof"+filePostfix)
	resFiles[types.ProofOfWork] = filepath.Join(h.config.OutDir, "POW"+filePostfix)
	resFiles[types.ConsistencyCheckChallenges] = filepath.Join(h.config.OutDir, "challenges"+filePostfix)

	aggFRI := []string{fmt.Sprintf("--proof=\"%v\"", resFiles[types.AggregatedFRIProof])}
	POW := []string{fmt.Sprintf("--proof-of-work-file=\"%v\"", resFiles[types.ProofOfWork])}
	consistencyChallenges := []string{fmt.Sprintf("--consistency-checks-challenges-file=\"%v\"", resFiles[types.ConsistencyCheckChallenges])}
	allArgs := slices.Concat(stage, assignmentTable, aggregatedChallenge, combinedQ, aggFRI, POW, consistencyChallenges)
	cmd := exec.Command(binary, allArgs...)
	return commandDescription{runCommand: cmd, expectedResult: resFiles}
}

func (h *taskHandler) makeConsistencyCheckCommand(task *types.Task) commandDescription {
	binary := h.config.ProofProducerBinary
	stage := []string{"--stage=\"consistency-checks\""}
	commitmentStateFile := collectDependencyFiles(task, types.PartialProve, types.CommitmentState)
	if len(commitmentStateFile) != 1 {
		panic("Insufficient input for a task")
	}
	commitmentState := []string{fmt.Sprintf("--commitment-state-file=\"%v\"", commitmentStateFile)}
	combinedQFile := collectDependencyFiles(task, types.CombinedQ, types.CombinedQPolynomial)
	if len(combinedQFile) != 1 {
		panic("Insufficient input for task")
	}
	combinedQ := []string{fmt.Sprintf("--combined-Q-polynomial-file=\"%v\"", combinedQFile)}
	consistencyChallengeFiles := collectDependencyFiles(task, types.AggregatedFRI, types.ConsistencyCheckChallenges)
	if len(consistencyChallengeFiles) != 1 {
		panic("Insufficient input for task")
	}
	consistencyChallenges := []string{fmt.Sprintf("--consistency-checks-challenges-file=\"%v\"", consistencyChallengeFiles)}

	outFile := fmt.Sprintf("LPC_consistency_check_proof.%v.%v.%v", circuitIdx(task.CircuitType), task.ShardId, task.BlockHash.String())
	outArg := []string{fmt.Sprintf("--proof=\"%v\"", filepath.Join(h.config.OutDir, outFile))}

	allArgs := slices.Concat(stage, commitmentState, combinedQ, consistencyChallenges, outArg)
	cmd := exec.Command(binary, allArgs...)
	return commandDescription{
		runCommand:     cmd,
		expectedResult: types.TaskResultFiles{types.LPCConsistencyCheckProof: outFile},
	}
}

func (h *taskHandler) makeMergeProofCommand(task *types.Task) commandDescription {
	binary := h.config.ProofProducerBinary
	stage := []string{"--stage=\"merge-proofs\""}
	partialProofFiles := collectDependencyFiles(task, types.PartialProve, types.PartialProof)
	if len(partialProofFiles) != int(types.CircuitAmount) {
		panic("Insufficient input for task")
	}
	partialProofs := append([]string{"--partial-proof"}, partialProofFiles...)

	LPCCheckFiles := collectDependencyFiles(task, types.FRIConsistencyChecks, types.LPCConsistencyCheckProof)
	if len(LPCCheckFiles) != int(types.CircuitAmount) {
		panic("Insufficient input for task")
	}
	LPCChecks := append([]string{"--initial-proof"}, LPCCheckFiles...)

	aggFRIFile := collectDependencyFiles(task, types.AggregatedFRI, types.AggregatedFRIProof)
	if len(aggFRIFile) != 1 {
		panic("Insufficient input for task")
	}
	aggFRI := []string{fmt.Sprintf("--aggregated-FRI-proof=\"%v\"", aggFRIFile)}

	outFile := fmt.Sprintf("final-proof.%v.%v", task.ShardId, task.BlockHash.String())
	outArg := []string{fmt.Sprintf("--proof=\"%v\"", filepath.Join(h.config.OutDir, outFile))}

	allArgs := slices.Concat(stage, partialProofs, LPCChecks, aggFRI, outArg)
	cmd := exec.Command(binary, allArgs...)
	return commandDescription{
		runCommand:     cmd,
		expectedResult: types.TaskResultFiles{types.FinalProof: outFile},
	}
}

func (h *taskHandler) makeCommandForTask(task *types.Task) commandDescription {
	switch task.TaskType {
	case types.PartialProve:
		return h.makePartialProofTaskCommand(task)
	case types.AggregatedChallenge:
		return h.makeAggregateChallengesTaskCommand(task)
	case types.CombinedQ:
		return h.makeCombinedQCommand(task)
	case types.AggregatedFRI:
		return h.makeAggregateFRICommand(task)
	case types.FRIConsistencyChecks:
		return h.makeConsistencyCheckCommand(task)
	case types.MergeProof:
		return h.makeMergeProofCommand(task)
	case types.ProofBlock:
		panic("ProofBlock task type is not supposed to be encountered in prover task handler")
	}
	panic("Unhandled task type")
}

func (h *taskHandler) Handle(ctx context.Context, executorId types.TaskExecutorId, task *types.Task) error {
	if task.TaskType == types.ProofBlock {
		return types.UnexpectedTaskType(task)
	}
	desc := h.makeCommandForTask(task)
	if desc.runCommand.Err != nil {
		return desc.runCommand.Err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	desc.runCommand.Stdout = &stdout
	desc.runCommand.Stderr = &stderr
	cmdString := strings.Join(desc.runCommand.Args, " ")
	startTime := time.Now()
	h.logger.Info().Msgf("Start task with id %v by command %v", task.Id.String(), cmdString)
	err := desc.runCommand.Run()
	if err != nil {
		h.logger.Info().Msgf("Task with id %v failed", task.Id.String())
		h.logger.Info().Msgf("Task execution stderr:\n%v\n", stderr.String())
		return err
	}
	h.logger.Info().Msgf("Task with id %v finished after %s", task.Id.String(), time.Since(startTime))
	taskResult := types.SuccessProverTaskResult(task.Id, executorId, task.TaskType, desc.expectedResult)
	return h.requestHandler.SetTaskResult(ctx, &taskResult)
}
