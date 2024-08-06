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
	var msg types.ExternalMessage
	if err := msg.UnmarshalSSZ(encoded); err != nil {
		return common.Hash{}, fmt.Errorf("failed to decode message: %w", err)
	}

	if msg.ChainId != types.DefaultChainId {
		return common.Hash{}, errInvalidChainId
	}

	shardId := msg.To.ShardId()
	if err := api.checkShard(shardId); err != nil {
		return common.Hash{}, err
	}

	msg2 := types.Message{
		Flags:     types.MessageFlagsFromKind(false, msg.Kind),
		To:        msg.To,
		From:      msg.To,
		ChainId:   msg.ChainId,
		Seqno:     msg.Seqno,
		Data:      msg.Data,
		Signature: msg.AuthData,
		FeeCredit: types.Gas(500_000).ToValue(types.DefaultGasPrice),
	}
	reason, err := api.msgPools[shardId].Add(ctx, []*types.Message{&msg2})
	if err != nil {
		return common.Hash{}, err
	}

	if reason[0] != msgpool.NotSet {
		return common.Hash{}, fmt.Errorf("message status: %s", reason[0])
	}

	return msg2.Hash(), nil
}
