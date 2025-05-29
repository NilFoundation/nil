package commands

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/exec"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
	"github.com/spf13/cobra"
)

type RollbackStateParams struct {
	exec.NoRefreshParams

	L1Endpoint         string
	PrivateKeyHex      string
	ContractAddressHex string
	TargetStateRootHex string
}

func hexOfLength(length int) *regexp.Regexp {
	pattern := fmt.Sprintf("(?:0x)?[0-9a-fA-F]{%d}", length)
	return regexp.MustCompile(pattern)
}

var (
	addressRegex    = hexOfLength(40)
	privateKeyRegex = hexOfLength(64)
	stateRootRegex  = hexOfLength(64)
)

func (p *RollbackStateParams) Validate() error {
	if _, err := url.Parse(p.L1Endpoint); err != nil {
		return fmt.Errorf("invalid L1 endpoint: %w", err)
	}

	if !privateKeyRegex.MatchString(p.PrivateKeyHex) {
		return errors.New("invalid private key format: must be a 32-byte hex string")
	}

	if !addressRegex.MatchString(p.ContractAddressHex) {
		return errors.New("invalid contract address format: must be a 20-byte hex string")
	}

	if !stateRootRegex.MatchString(p.TargetStateRootHex) {
		return errors.New("invalid state root format: must be a 32-byte hex string")
	}

	return nil
}

type rollbackState struct {
	logger logging.Logger
}

func NewRollbackStateCmd(logger logging.Logger) *rollbackState {
	return &rollbackState{
		logger: logger,
	}
}

func (c *rollbackState) Build() (*cobra.Command, error) {
	params := &RollbackStateParams{}

	cmd := &cobra.Command{
		Use:   "rollback-state",
		Short: "Rollback L1 state to specified root",
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := exec.NewExecutor(os.Stdout, c.logger, params)
			return executor.Run(func(ctx context.Context) (exec.CmdOutput, error) {
				return c.rollbackState(ctx, params)
			})
		},
	}

	endpointFlag := "l1-endpoint"
	cmd.Flags().StringVar(
		&params.L1Endpoint,
		endpointFlag,
		params.L1Endpoint,
		"L1 endpoint")
	privateKeyFlag := "l1-private-key"
	cmd.Flags().StringVar(
		&params.PrivateKeyHex,
		privateKeyFlag,
		params.PrivateKeyHex,
		"L1 account private key")
	addressFlag := "l1-contract-address"
	cmd.Flags().StringVar(
		&params.ContractAddressHex,
		addressFlag,
		params.ContractAddressHex,
		"L1 update state contract address")
	targetRootFlag := "target-root"
	cmd.Flags().StringVar(
		&params.TargetStateRootHex,
		"target-root",
		params.TargetStateRootHex,
		"target state root in HEX")

	// make all flags required
	for _, flagId := range []string{endpointFlag, privateKeyFlag, addressFlag, targetRootFlag} {
		if err := cmd.MarkFlagRequired(flagId); err != nil {
			return nil, err
		}
	}

	return cmd, nil
}

func (c *rollbackState) rollbackState(ctx context.Context, params *RollbackStateParams) (exec.CmdOutput, error) {
	config := rollupcontract.NewWrapperConfig(
		params.L1Endpoint,
		params.PrivateKeyHex,
		params.ContractAddressHex,
		rollupcontract.DefaultRequestTimeout,
		false,
	)

	wrapper, err := rollupcontract.NewWrapper(ctx, config, c.logger)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("reset failed on wrapper creation: %w", err)
	}

	targetStateRoot := common.HexToHash(params.TargetStateRootHex)
	err = wrapper.RollbackState(ctx, targetStateRoot)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("reset failed: %w", err)
	}

	return "Reset is completed successfully", nil
}
