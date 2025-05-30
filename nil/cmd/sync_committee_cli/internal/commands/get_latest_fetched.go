package commands

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/exec"
	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/output"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/debug"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/spf13/cobra"
)

type getLatestFetched struct {
	logger logging.Logger
}

func NewGetLatestFetchedCmd(logger logging.Logger) *getLatestFetched {
	return &getLatestFetched{
		logger: logger,
	}
}

func (c *getLatestFetched) Build() (*cobra.Command, error) {
	paramsWithEndpoint := defaultParamsWithEndpoint()

	cmd := &cobra.Command{
		Use:   "get-latest-fetched",
		Short: "Retrieves references to the latest fetched blocks for all shards",
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := exec.NewExecutor(os.Stdout, c.logger, paramsWithEndpoint)
			client := debug.NewBlocksClient(paramsWithEndpoint.RpcEndpoint, c.logger)
			return executor.Run(func(ctx context.Context) (exec.CmdOutput, error) {
				return c.getLatestFetched(ctx, client)
			})
		},
	}

	paramsWithEndpoint.bind(cmd)
	return cmd, nil
}

func (c *getLatestFetched) getLatestFetched(ctx context.Context, client public.BlockDebugApi) (exec.CmdOutput, error) {
	latestFetched, err := client.GetLatestFetched(ctx)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to get state root data: %w", err)
	}

	table, err := c.dataAsTable(latestFetched)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to build state root data table: %w", err)
	}

	return table.AsCmdOutput(), nil
}

func (*getLatestFetched) dataAsTable(latestFetched public.BlockRefs) (*output.Table, error) {
	headers := output.NewTableRowStr("Shard Id", "Block Hash", "Block Number")

	rows := make([]output.TableRow, 0, len(latestFetched))

	sortedRefs := slices.SortedFunc(
		maps.Values(latestFetched),
		func(l public.BlockRef, r public.BlockRef) int { return cmp.Compare(l.ShardId, r.ShardId) },
	)

	for _, ref := range sortedRefs {
		row := output.NewTableRow(ref.ShardId, ref.Hash, ref.Number)
		rows = append(rows, row)
	}

	return output.NewTable(headers, rows)
}
