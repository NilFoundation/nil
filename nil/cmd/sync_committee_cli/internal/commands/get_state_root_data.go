package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/exec"
	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/output"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/debug"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/spf13/cobra"
)

type getStateRootData struct {
	logger logging.Logger
}

func NewGetStateRootDataCmd(logger logging.Logger) *getStateRootData {
	return &getStateRootData{
		logger: logger,
	}
}

func (c *getStateRootData) Build() (*cobra.Command, error) {
	paramsWithEndpoint := defaultParamsWithEndpoint()

	cmd := &cobra.Command{
		Use:   "get-state-root-data",
		Short: "Retrieve the current state root data, including the L1 state root and the local state root",
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := exec.NewExecutor(os.Stdout, c.logger, paramsWithEndpoint)
			client := debug.NewBlocksClient(paramsWithEndpoint.RpcEndpoint, c.logger)
			return executor.Run(func(ctx context.Context) (exec.CmdOutput, error) {
				return c.getStateRootData(ctx, client)
			})
		},
	}

	paramsWithEndpoint.bind(cmd)
	return cmd, nil
}

func (c *getStateRootData) getStateRootData(ctx context.Context, client public.BlockDebugApi) (exec.CmdOutput, error) {
	stateRootData, err := client.GetStateRootData(ctx)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to get state root data: %w", err)
	}

	table, err := c.dataAsTable(stateRootData)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to build state root data table: %w", err)
	}

	return table.AsCmdOutput(), nil
}

func (*getStateRootData) dataAsTable(data *public.StateRootData) (*output.Table, error) {
	header := output.NewTableRowStr("Field", "Value")

	rows := []output.TableRow{
		output.NewTableRow(output.StrCell("L1 State Root"), data.L1StateRoot),
		output.NewTableRow(output.StrCell("Local State Root"), data.LocalStateRoot),
	}

	return output.NewTable(header, rows)
}
