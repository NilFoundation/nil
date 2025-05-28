package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/exec"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/debug"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/spf13/cobra"
)

type GetBatchParams struct {
	exec.Params
	BatchId public.BatchId
}

type getBatch struct {
	logger logging.Logger
}

func NewGetBatchCmd(logger logging.Logger) *getBatch {
	return &getBatch{
		logger: logger,
	}
}

func (c *getBatch) Build() (*cobra.Command, error) {
	paramsWithEndpoint := defaultParamsWithEndpoint()

	cmdParams := &GetBatchParams{
		Params: paramsWithEndpoint.Params,
	}

	cmd := &cobra.Command{
		Use:   "get-batch",
		Short: "Retrieves detailed information about a specific block batch identified by the given BatchId",
		RunE: func(*cobra.Command, []string) error {
			executor := exec.NewExecutor(os.Stdout, c.logger, cmdParams)
			client := debug.NewBlocksClient(paramsWithEndpoint.RpcEndpoint, c.logger)
			return executor.Run(func(ctx context.Context) (exec.CmdOutput, error) {
				return c.getBatch(ctx, cmdParams.BatchId, client)
			})
		},
	}

	paramsWithEndpoint.bind(cmd)

	const batchIdFlag = "batch-id"
	cmd.Flags().Var(&cmdParams.BatchId, batchIdFlag, "batch identifier")
	if err := cmd.MarkFlagRequired(batchIdFlag); err != nil {
		return nil, err
	}

	return cmd, nil
}

func (c *getBatch) getBatch(
	ctx context.Context, batchId public.BatchId, api public.BlockDebugApi,
) (exec.CmdOutput, error) {
	batch, err := api.GetBatchView(ctx, batchId)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to get batches from debug API: %w", err)
	}

	if batch == nil {
		return exec.EmptyOutput, fmt.Errorf(
			"%w: batch with id=%s is not found in the local storage", exec.ErrNoDataFound, batchId,
		)
	}

	jsonBytes, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("batch json marshaling failed: %w", err)
	}

	return string(jsonBytes), nil
}
