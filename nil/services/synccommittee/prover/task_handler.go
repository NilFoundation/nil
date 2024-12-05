package prover

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
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
	runCommand            *exec.Cmd
	expectedResult        types.TaskResultAddresses
	binaryExpectedResults types.TaskResultData
}

func circuitTypeToArg(ct types.CircuitType) string {
	switch ct {
	case types.None:
		return "none"
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
	case types.None:
		return 0
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

func collectDependencyFiles(task *types.Task, dependencyType types.TaskType, resultType types.ProverResultType) ([]string, error) {
	depFiles := []string{}
	for _, res := range task.DependencyResults {
		if res.Type == dependencyType {
			path, ok := res.DataAddresses[resultType]
			if !ok {
				return depFiles, errors.New("Inconsistent task " + task.Id.String() +
					", dependencyType " + dependencyType.String() + " has no expected result " + resultType.String())
			}
			depFiles = append(depFiles, path)
		}
	}
	return depFiles, nil
}

func insufficientTaskInputMsg(task *types.Task, dependencyType string, expected int, actual int) string {
	return "Insufficient input for task " + task.Id.String() +
		" type " + task.TaskType.String() + "on " + dependencyType + "dependency: expected " + strconv.Itoa(expected) +
		" actual " + strconv.Itoa(actual)
}

func (h *taskHandler) makePartialProofTaskCommand(task *types.Task) commandDescription {
	binary := h.config.AssignerBinary
	blockData := []string{"--shard-id", task.ShardId.String(), "--block-hash", task.BlockHash.String()}
	outDir := []string{"--path", h.config.OutDir}
	circuit := []string{"--target-circuits", circuitTypeToArg(task.CircuitType)}
	allArgs := slices.Concat(blockData, outDir, circuit)
	cmd := exec.Command(binary, allArgs...)
	resFiles := make(types.TaskResultAddresses)
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
	var cmd exec.Cmd
	inputFiles, err := collectDependencyFiles(task, types.PartialProve, types.PartialProofChallenges)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(inputFiles) != int(types.CircuitAmount) {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "PartialProofChallenges", int(types.CircuitAmount), len(inputFiles)))
		return commandDescription{runCommand: &cmd}
	}
	inputs := append([]string{"--input-challenge-files"}, inputFiles...)
	outFile := fmt.Sprintf("aggregated_challenges.%v.%v", task.ShardId, task.BlockHash.String())
	outArg := []string{fmt.Sprintf("--aggregated-challenge-file=\"%v\"", filepath.Join(h.config.OutDir, outFile))}
	allArgs := slices.Concat(stage, inputs, outArg)
	return commandDescription{
		runCommand:     exec.Command(binary, allArgs...),
		expectedResult: types.TaskResultAddresses{types.AggregatedChallenges: outFile},
	}
}

func (h *taskHandler) makeCombinedQCommand(task *types.Task) commandDescription {
	binary := h.config.ProofProducerBinary
	stage := []string{"--stage=\"compute-combined-Q\""}
	var cmd exec.Cmd
	commitmentStateFile, err := collectDependencyFiles(task, types.PartialProve, types.CommitmentState)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(commitmentStateFile) != 1 {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "CommitmentState", 1, len(commitmentStateFile)))
		return commandDescription{runCommand: &cmd}
	}
	commitmentState := []string{fmt.Sprintf("--commitment-state-file=\"%v\"", commitmentStateFile[0])}

	aggChallengesFile, err := collectDependencyFiles(task, types.AggregatedChallenge, types.AggregatedChallenges)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(aggChallengesFile) != 1 {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "AggregatedChallenges", 1, len(aggChallengesFile)))
		return commandDescription{runCommand: &cmd}
	}
	aggregateChallenges := []string{fmt.Sprintf("--aggregated-challenge-file=\"%v\"", aggChallengesFile[0])}

	startingPower := []string{"--combined-Q-starting-power=0"} // TODO: compute it properly from dependencies
	outFile := fmt.Sprintf("combined_Q.%v.%v.%v", circuitIdx(task.CircuitType), task.ShardId, task.BlockHash.String())
	outArg := []string{fmt.Sprintf("--combined-Q-polynomial-file=\"%v\"", filepath.Join(h.config.OutDir, outFile))}

	allArgs := slices.Concat(stage, commitmentState, aggregateChallenges, startingPower, outArg)
	return commandDescription{
		runCommand:     exec.Command(binary, allArgs...),
		expectedResult: types.TaskResultAddresses{types.CombinedQPolynomial: outFile},
	}
}

func (h *taskHandler) makeAggregateFRICommand(task *types.Task) commandDescription {
	binary := h.config.ProofProducerBinary
	stage := []string{"--stage=\"aggregated-FRI\""}
	var cmd exec.Cmd
	assignmentTableFile, err := collectDependencyFiles(task, types.PartialProve, types.AssignmentTableDescription)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(assignmentTableFile) != int(types.CircuitAmount) {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "AssignmentTableDescription", int(types.CircuitAmount), len(assignmentTableFile)))
		return commandDescription{runCommand: &cmd}
	}
	assignmentTable := []string{fmt.Sprintf("--assignment-description-file=\"%v\"", assignmentTableFile[0])}

	aggChallengeFile, err := collectDependencyFiles(task, types.AggregatedChallenge, types.AggregatedChallenges)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(aggChallengeFile) != 1 {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "AggregatedChallenges", 1, len(aggChallengeFile)))
		return commandDescription{runCommand: &cmd}
	}
	aggregatedChallenge := []string{fmt.Sprintf("--aggregated-challenge-file=\"%v\"", aggChallengeFile[0])}

	combinedQFiles, err := collectDependencyFiles(task, types.CombinedQ, types.CombinedQPolynomial)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(combinedQFiles) != int(types.CircuitAmount) {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "CombinedQPolynomial", int(types.CircuitAmount), len(combinedQFiles)))
		return commandDescription{runCommand: &cmd}
	}
	combinedQ := append([]string{"--input-combined-Q-polynomial-files"}, combinedQFiles...)

	resFiles := make(types.TaskResultAddresses)
	filePostfix := fmt.Sprintf(".%v.%v", task.ShardId, task.BlockHash.String())
	resFiles[types.AggregatedFRIProof] = filepath.Join(h.config.OutDir, "aggregated_FRI_proof"+filePostfix)
	resFiles[types.ProofOfWork] = filepath.Join(h.config.OutDir, "POW"+filePostfix)
	resFiles[types.ConsistencyCheckChallenges] = filepath.Join(h.config.OutDir, "challenges"+filePostfix)

	aggFRI := []string{fmt.Sprintf("--proof=\"%v\"", resFiles[types.AggregatedFRIProof])}
	POW := []string{fmt.Sprintf("--proof-of-work-file=\"%v\"", resFiles[types.ProofOfWork])}
	consistencyChallenges := []string{fmt.Sprintf("--consistency-checks-challenges-file=\"%v\"", resFiles[types.ConsistencyCheckChallenges])}
	allArgs := slices.Concat(stage, assignmentTable, aggregatedChallenge, combinedQ, aggFRI, POW, consistencyChallenges)
	return commandDescription{runCommand: exec.Command(binary, allArgs...), expectedResult: resFiles}
}

func (h *taskHandler) makeConsistencyCheckCommand(task *types.Task) commandDescription {
	binary := h.config.ProofProducerBinary
	stage := []string{"--stage=\"consistency-checks\""}
	var cmd exec.Cmd
	commitmentStateFile, err := collectDependencyFiles(task, types.PartialProve, types.CommitmentState)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(commitmentStateFile) != 1 {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "CommitmentState", 1, len(commitmentStateFile)))
		return commandDescription{runCommand: &cmd}
	}
	commitmentState := []string{fmt.Sprintf("--commitment-state-file=\"%v\"", commitmentStateFile)}
	combinedQFile, err := collectDependencyFiles(task, types.CombinedQ, types.CombinedQPolynomial)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(combinedQFile) != 1 {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "CombinedQPolynomial", 1, len(combinedQFile)))
		return commandDescription{runCommand: &cmd}
	}
	combinedQ := []string{fmt.Sprintf("--combined-Q-polynomial-file=\"%v\"", combinedQFile)}
	consistencyChallengeFiles, err := collectDependencyFiles(task, types.AggregatedFRI, types.ConsistencyCheckChallenges)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(consistencyChallengeFiles) != 1 {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "ConsistencyCheckChallenges", 1, len(consistencyChallengeFiles)))
		return commandDescription{runCommand: &cmd}
	}
	consistencyChallenges := []string{fmt.Sprintf("--consistency-checks-challenges-file=\"%v\"", consistencyChallengeFiles)}

	outFile := fmt.Sprintf("LPC_consistency_check_proof.%v.%v.%v", circuitIdx(task.CircuitType), task.ShardId, task.BlockHash.String())
	outArg := []string{fmt.Sprintf("--proof=\"%v\"", filepath.Join(h.config.OutDir, outFile))}

	allArgs := slices.Concat(stage, commitmentState, combinedQ, consistencyChallenges, outArg)
	return commandDescription{
		runCommand:     exec.Command(binary, allArgs...),
		expectedResult: types.TaskResultAddresses{types.LPCConsistencyCheckProof: outFile},
	}
}

func (h *taskHandler) makeMergeProofCommand(task *types.Task) commandDescription {
	binary := h.config.ProofProducerBinary
	stage := []string{"--stage=\"merge-proofs\""}
	var cmd exec.Cmd
	partialProofFiles, err := collectDependencyFiles(task, types.PartialProve, types.PartialProof)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(partialProofFiles) != int(types.CircuitAmount) {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "PartialProof", int(types.CircuitAmount), len(partialProofFiles)))
		return commandDescription{runCommand: &cmd}
	}
	partialProofs := append([]string{"--partial-proof"}, partialProofFiles...)

	LPCCheckFiles, err := collectDependencyFiles(task, types.FRIConsistencyChecks, types.LPCConsistencyCheckProof)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(LPCCheckFiles) != int(types.CircuitAmount) {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "LPCConsistencyCheckProof", int(types.CircuitAmount), len(LPCCheckFiles)))
		return commandDescription{runCommand: &cmd}
	}
	LPCChecks := append([]string{"--initial-proof"}, LPCCheckFiles...)

	aggFRIFile, err := collectDependencyFiles(task, types.AggregatedFRI, types.AggregatedFRIProof)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	if len(aggFRIFile) != 1 {
		cmd.Err = errors.New(insufficientTaskInputMsg(task, "AggregatedFRIProof", int(types.CircuitAmount), len(aggFRIFile)))
		return commandDescription{runCommand: &cmd}
	}
	aggFRI := []string{fmt.Sprintf("--aggregated-FRI-proof=\"%v\"", aggFRIFile)}

	outFile := fmt.Sprintf("final-proof.%v.%v", task.ShardId, task.BlockHash.String())
	outArg := []string{fmt.Sprintf("--proof=\"%v\"", filepath.Join(h.config.OutDir, outFile))}

	allArgs := slices.Concat(stage, partialProofs, LPCChecks, aggFRI, outArg)
	return commandDescription{
		runCommand:            exec.Command(binary, allArgs...),
		expectedResult:        types.TaskResultAddresses{types.FinalProof: outFile},
		binaryExpectedResults: task.BlockHash.Bytes(),
	}
}

func (h *taskHandler) makeAggregateProofCommand(task *types.Task) commandDescription {
	binary := h.config.ProofProducerBinary
	stage := []string{"--stage=\"aggregate-proofs\""}
	var cmd exec.Cmd
	blockProofFiles, err := collectDependencyFiles(task, types.MergeProof, types.FinalProof)
	if err != nil {
		cmd.Err = err
		return commandDescription{runCommand: &cmd}
	}
	blockProofs := append([]string{"--block-proof"}, blockProofFiles...)

	outFile := fmt.Sprintf("aggregated-proof.%v.%v", task.ShardId, task.BlockHash.String())
	outArg := []string{fmt.Sprintf("--proof=\"%v\"", filepath.Join(h.config.OutDir, outFile))}

	allArgs := slices.Concat(stage, blockProofs, outArg)
	var aggregatedProof []byte
	for _, dependency := range task.DependencyResults {
		aggregatedProof = append(aggregatedProof, dependency.Data...)
	}
	return commandDescription{
		runCommand:            exec.Command(binary, allArgs...),
		expectedResult:        types.TaskResultAddresses{types.AggregatedProof: outFile},
		binaryExpectedResults: aggregatedProof,
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
	case types.AggregateProofs:
		return h.makeAggregateProofCommand(task)
	case types.ProofBlock:
		var cmd exec.Cmd
		cmd.Err = errors.New("ProofBlock task type is not supposed to be encountered in prover task handler for task " + task.Id.String() +
			" type " + task.TaskType.String())
		return commandDescription{runCommand: &cmd}
	default:
		var cmd exec.Cmd
		cmd.Err = errors.New("Unknown type for task " + task.Id.String() +
			" type " + task.TaskType.String())
		return commandDescription{runCommand: &cmd}
	}
}

func (h *taskHandler) Handle(ctx context.Context, executorId types.TaskExecutorId, task *types.Task) error {
	if task.TaskType == types.ProofBlock {
		err := types.UnexpectedTaskType(task)
		taskResult := types.FailureProverTaskResult(task.Id, executorId, fmt.Errorf("failed to create command for task: %w", err))
		h.logger.Error().Msgf("failed to create command for task with id=%s: %v", task.Id, err)
		return h.requestHandler.SetTaskResult(ctx, &taskResult)
	}
	desc := h.makeCommandForTask(task)
	if desc.runCommand.Err != nil {
		taskResult := types.FailureProverTaskResult(task.Id, executorId, fmt.Errorf("failed to create command for task: %w", desc.runCommand.Err))
		h.logger.Error().Msgf("failed to create command for task with id=%s: %v", task.Id, desc.runCommand.Err)
		return h.requestHandler.SetTaskResult(ctx, &taskResult)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	desc.runCommand.Stdout = &stdout
	desc.runCommand.Stderr = &stderr
	cmdString := strings.Join(desc.runCommand.Args, " ")
	startTime := time.Now()
	h.logger.Info().Msgf("Start task %v with id %v for prove block %v from shard %d in batch %d by command %v", task.TaskType.String(), task.Id.String(), task.BlockHash.String(), task.ShardId, task.BatchId, cmdString)
	err := desc.runCommand.Run()
	if err != nil {
		taskResult := types.FailureProverTaskResult(task.Id, executorId, fmt.Errorf("task execution failed: %w", err))
		h.logger.Error().Msgf("Task with id %v failed", task.Id.String())
		h.logger.Error().Msgf("Task execution stderr:\n%v\n", stderr.String())
		return h.requestHandler.SetTaskResult(ctx, &taskResult)
	}
	h.logger.Info().Msgf("Task with id %v finished after %s", task.Id.String(), time.Since(startTime))
	taskResult := types.SuccessProverTaskResult(task.Id, executorId, task.TaskType, desc.expectedResult, desc.binaryExpectedResults)
	return h.requestHandler.SetTaskResult(ctx, &taskResult)
}
