package commands

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
)

type ResetStateParams struct {
	Endpoint           string
	PrivateKeyHex      string
	ContractAddressHex string
	TargetStateRoot    string
}

func ResetState(ctx context.Context, params *ResetStateParams, logger logging.Logger) error {
	wrapper, err := rollupcontract.NewWrapper(ctx, rollupcontract.WrapperConfig{
		Endpoint:           params.Endpoint,
		PrivateKeyHex:      params.PrivateKeyHex,
		ContractAddressHex: params.ContractAddressHex,
	}, logger)
	if err != nil {
		return fmt.Errorf("reset failed on wrapper creation: %w", err)
	}

	targetStateRoot := common.HexToHash(params.TargetStateRoot)
	return wrapper.ResetState(ctx, targetStateRoot)
}
