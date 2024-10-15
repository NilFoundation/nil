package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/types"
)

func (api *APIImpl) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	return api.rawapi.GetShardIdList(ctx)
}
