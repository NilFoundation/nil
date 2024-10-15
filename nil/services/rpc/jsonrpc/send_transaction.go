package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
)

// SendRawTransaction implements eth_sendRawTransaction. Creates new message or a contract creation for previously-signed message.
func (api *APIImpl) SendRawTransaction(ctx context.Context, encoded hexutil.Bytes) (common.Hash, error) {
	var extMsg types.ExternalMessage
	if err := extMsg.UnmarshalSSZ(encoded); err != nil {
		return common.EmptyHash, fmt.Errorf("failed to decode message: %w", err)
	}

	shardId := extMsg.To.ShardId()
	reason, err := api.rawapi.SendMessage(ctx, shardId, []byte(encoded))
	if err != nil {
		return common.EmptyHash, err
	}

	if reason != msgpool.NotSet {
		return common.Hash{}, fmt.Errorf("message status: %s", reason)
	}
	return extMsg.Hash(), nil
}
