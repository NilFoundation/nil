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
		return common.Hash{}, fmt.Errorf("failed to decode message: %w", err)
	}

	if extMsg.ChainId != types.DefaultChainId {
		return common.Hash{}, errInvalidChainId
	}

	shardId := extMsg.To.ShardId()
	if err := api.checkShard(shardId); err != nil {
		return common.Hash{}, err
	}

	msg := extMsg.ToMessage()
	reason, err := api.msgPools[shardId].Add(ctx, msg)
	if err != nil {
		return common.Hash{}, err
	}

	if reason[0] != msgpool.NotSet {
		return common.Hash{}, fmt.Errorf("message status: %s", reason[0])
	}

	return msg.Hash(), nil
}
