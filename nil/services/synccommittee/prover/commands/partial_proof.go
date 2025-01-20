package commands

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer"
	"github.com/rs/zerolog"
)

type partialProofCmd struct {
	client         client.Client
	logger         zerolog.Logger
	nilRpcEndpoint string
	cmdCommon
}

func NewPartialProofCmd(config CommandConfig, logger zerolog.Logger) Command {
	return &partialProofCmd{
		client:         rpc.NewRetryClient(config.NilRpcEndpoint, logging.NewLogger("client")),
		nilRpcEndpoint: config.NilRpcEndpoint,
		cmdCommon:      makeCmdCommon(config),
		logger:         logger,
	}
}

var _ BeforeCommandExecuted = new(partialProofCmd)

func (cmd *partialProofCmd) MakeCommandDefinition(task *types.Task) (*CommandDefinition, error) {
	resultFiles := make(types.TaskOutputArtifacts)
	proofProducerBinary := cmd.binary
	stage := []string{"--stage", "fast-generate-partial-proof"}
	filePostfix := fmt.Sprintf(".%v.%v.%v", circuitIdx(task.CircuitType), task.ShardId, task.BlockHash.String())
	circuitName := []string{"--circuit-name", circuitTypeToArg(task.CircuitType)}

	assignmentDescFile := filepath.Join(cmd.outDir, "assignment_table_description"+filePostfix)
	assignmentDescArg := []string{"--assignment-description-file", assignmentDescFile}
	resultFiles[types.AssignmentTableDescription] = assignmentDescFile

	traceArg := []string{"--trace", cmd.getTraceFileName(task)}

	partialProofFile := filepath.Join(cmd.outDir, "partial_proof"+filePostfix)
	partialProofArg := []string{"--proof", partialProofFile}
	resultFiles[types.PartialProof] = partialProofFile

	challengeFile := filepath.Join(cmd.outDir, "challenge"+filePostfix)
	challengeArg := []string{"--challenge-file", challengeFile}
	resultFiles[types.PartialProofChallenges] = challengeFile

	thetaPowerFile := filepath.Join(cmd.outDir, "theta_power"+filePostfix)
	thetaPowerArg := []string{"--theta-power-file", thetaPowerFile}
	resultFiles[types.ThetaPower] = thetaPowerFile

	commonDataFile := filepath.Join(cmd.outDir, "preprocessed_common_data"+filePostfix)
	commonDataArg := []string{"--common-data", commonDataFile}
	resultFiles[types.PreprocessedCommonData] = commonDataFile

	commitmentStateFile := filepath.Join(cmd.outDir, "commitment_state"+filePostfix)
	commitmentStateArg := []string{"--updated-commitment-state-file", commitmentStateFile}
	resultFiles[types.CommitmentState] = commitmentStateFile

	allArgs := slices.Concat(stage, circuitName, assignmentDescArg, traceArg, partialProofArg, challengeArg, thetaPowerArg, commitmentStateArg, commonDataArg)
	execCmd := exec.Command(proofProducerBinary, allArgs...)
	if execCmd.Err != nil {
		return nil, execCmd.Err
	}

	return &CommandDefinition{
		ExecCommands:   []*exec.Cmd{execCmd},
		ExpectedResult: resultFiles,
	}, nil
}

func (cmd *partialProofCmd) getTraceFileName(task *types.Task) string {
	return filepath.Join(cmd.outDir, fmt.Sprintf("trace.%v.%v", task.ShardId, task.BlockHash))
}

func (cmd *partialProofCmd) BeforeCommandExecuted(ctx context.Context, task *types.Task, results types.TaskOutputArtifacts) error {
	traceFileName := cmd.getTraceFileName(task)
	cmd.logger.Info().Msgf("Tracer arguments: trace --nil-endpoint %v %v %v %v", cmd.nilRpcEndpoint, traceFileName, task.ShardId, task.BlockHash.String())

	tracerConfig := tracer.TraceConfig{
		MarshalMode:  tracer.MarshalModeBinary,
		ShardID:      task.ShardId,
		BaseFileName: traceFileName,
		BlockIDs:     []transport.BlockReference{transport.HashBlockReference(task.BlockHash)},
	}
	return tracer.GenerateTrace(ctx, cmd.client, &tracerConfig)
}
