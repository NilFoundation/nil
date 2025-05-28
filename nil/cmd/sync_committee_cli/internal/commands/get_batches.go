package commands

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/exec"
	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/output"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/debug"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/spf13/cobra"
)

type GetBatchesParams struct {
	exec.Params
	public.BatchDebugRequest
}

func (p *GetBatchesParams) Validate() error {
	return errors.Join(
		p.Params.Validate(),
		p.BatchDebugRequest.Validate(),
	)
}

type getBatches struct {
	logger logging.Logger
}

func NewGetBatchesCmd(logger logging.Logger) *getBatches {
	return &getBatches{
		logger: logger,
	}
}

func (c *getBatches) Build() (*cobra.Command, error) {
	paramsWithEndpoint := defaultParamsWithEndpoint()

	cmdParams := &GetBatchesParams{
		Params:            paramsWithEndpoint.Params,
		BatchDebugRequest: public.DefaultBatchDebugRequest(),
	}

	cmd := &cobra.Command{
		Use:   "get-batches",
		Short: "Retrieve a list of batches currently persisted in the storage based on the given parameters",
		RunE: func(*cobra.Command, []string) error {
			executor := exec.NewExecutor(os.Stdout, c.logger, cmdParams)
			client := debug.NewBlocksClient(paramsWithEndpoint.RpcEndpoint, c.logger)
			return executor.Run(func(ctx context.Context) (exec.CmdOutput, error) {
				return c.getBatches(ctx, cmdParams.BatchDebugRequest, client)
			})
		},
	}

	paramsWithEndpoint.bind(cmd)
	bindListRequest(&cmdParams.ListRequest, cmd)
	return cmd, nil
}

func (c *getBatches) getBatches(
	ctx context.Context, request public.BatchDebugRequest, api public.BlockDebugApi,
) (exec.CmdOutput, error) {
	batches, err := api.GetBatchViews(ctx, request)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to get batches from debug API: %w", err)
	}

	if len(batches) == 0 {
		return exec.EmptyOutput, fmt.Errorf("%w: no batches are currently created", exec.ErrNoDataFound)
	}

	tasksTable, err := c.dataAsTable(batches)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to build tasks table: %w", err)
	}
	tableOutput := tasksTable.AsCmdOutput()
	return tableOutput, nil
}

func (c *getBatches) dataAsTable(batches []*public.BatchViewCompact) (*output.Table, error) {
	fields := AllBatchFields()

	rows := make([]output.TableRow, 0, len(batches))
	for _, batch := range batches {
		row := c.batchAsRow(batch, fields)
		rows = append(rows, row)
	}

	return output.NewTable(fields, rows)
}

func (c *getBatches) batchAsRow(batch *public.BatchViewCompact, fieldsToInclude []BatchField) output.TableRow {
	row := make(output.TableRow, 0, len(fieldsToInclude))

	for _, fieldName := range fieldsToInclude {
		fieldData := BatchViewFields[fieldName]
		strValue := fieldData.Getter(batch)
		row = append(row, strValue)
	}

	return row
}
