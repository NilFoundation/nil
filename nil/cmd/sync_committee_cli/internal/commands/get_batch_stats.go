package commands

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/exec"
	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/output"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/debug"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/spf13/cobra"
)

type getBatchStats struct {
	logger logging.Logger
}

func NewGetBatchStatsCmd(logger logging.Logger) *getBatchStats {
	return &getBatchStats{
		logger: logger,
	}
}

func (c *getBatchStats) Build() (*cobra.Command, error) {
	paramsWithEndpoint := defaultParamsWithEndpoint()

	cmd := &cobra.Command{
		Use:   "get-batch-stats",
		Short: "Retrieve statistics about batches currently persisted in the storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := exec.NewExecutor(os.Stdout, c.logger, paramsWithEndpoint)
			client := debug.NewBlocksClient(paramsWithEndpoint.RpcEndpoint, c.logger)
			return executor.Run(func(ctx context.Context) (exec.CmdOutput, error) {
				return c.getBatchStats(ctx, client)
			})
		},
	}

	paramsWithEndpoint.bind(cmd)
	return cmd, nil
}

func (c *getBatchStats) getBatchStats(ctx context.Context, client public.BlockDebugApi) (exec.CmdOutput, error) {
	batchStats, err := client.GetBatchStats(ctx)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to get batch stats: %w", err)
	}

	table, err := c.dataAsTable(batchStats)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to build batch stats table: %w", err)
	}

	return table.AsCmdOutput(), nil
}

func (c *getBatchStats) dataAsTable(stats *public.BatchStats) (*output.Table, error) {
	header := output.NewTableRowStr("Field", "Value")

	rows := []output.TableRow{
		output.NewTableRowStr("Total Batches Created", strconv.Itoa(stats.TotalCount)),
		output.NewTableRowStr("Sealed Batches Count", strconv.Itoa(stats.SealedCount)),
		output.NewTableRowStr("Proved Batches Count", strconv.Itoa(stats.ProvedCount)),
	}

	return output.NewTable(header, rows)
}
