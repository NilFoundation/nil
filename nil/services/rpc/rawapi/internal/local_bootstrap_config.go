package internal

import (
	"context"

	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

func (api *localShardApiRo) GetBootstrapConfig(ctx context.Context) (*rpctypes.BootstrapConfig, error) {
	return api.bootstrapConfig, nil
}
