package commands

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/exec"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core"
	"github.com/spf13/cobra"
)

type ParamsWithEndpoint struct {
	exec.Params
	RpcEndpoint string
}

func (p *ParamsWithEndpoint) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&p.RpcEndpoint, "endpoint", p.RpcEndpoint, "target rpc endpoint")
	cmd.Flags().BoolVar(&p.AutoRefresh, "refresh", p.AutoRefresh, "should the received data be refreshed")
	cmd.Flags().DurationVar(
		&p.RefreshInterval,
		"refresh-interval",
		p.RefreshInterval,
		fmt.Sprintf("refresh interval, min value is %s", exec.RefreshIntervalMinimal),
	)
}

func defaultParamsWithEndpoint() ParamsWithEndpoint {
	return ParamsWithEndpoint{
		Params:      exec.DefaultExecutorParams(),
		RpcEndpoint: core.DefaultOwnRpcEndpoint,
	}
}
