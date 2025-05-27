package commands

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
)

type RollbackStateParams struct {
	NoRefresh

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

func RollbackState(ctx context.Context, params *RollbackStateParams, logger logging.Logger) (CmdOutput, error) {
	config := rollupcontract.NewWrapperConfig(
		params.L1Endpoint,
		params.PrivateKeyHex,
		params.ContractAddressHex,
		rollupcontract.DefaultRequestTimeout,
		false,
	)

	wrapper, err := rollupcontract.NewWrapper(ctx, config, logger)
	if err != nil {
		return EmptyOutput, fmt.Errorf("reset failed on wrapper creation: %w", err)
	}

	targetStateRoot := common.HexToHash(params.TargetStateRootHex)
	err = wrapper.RollbackState(ctx, targetStateRoot)
	if err != nil {
		return EmptyOutput, fmt.Errorf("reset failed: %w", err)
	}

	return "Reset is completed successfully", nil
}
