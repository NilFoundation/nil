package rawapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
)

func (api *LocalShardApi) SendMessage(ctx context.Context, encoded []byte) (msgpool.DiscardReason, error) {
	if api.msgpool == nil {
		return 0, errors.New("message pool is not available")
	}

	var extMsg types.ExternalMessage
	if err := extMsg.UnmarshalSSZ(encoded); err != nil {
		return 0, fmt.Errorf("failed to decode message: %w", err)
	}

	reasons, err := api.msgpool.Add(ctx, extMsg.ToMessage())
	if err != nil {
		return 0, err
	}
	return reasons[0], nil
}
