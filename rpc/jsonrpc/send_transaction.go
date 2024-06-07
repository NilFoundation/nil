package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/features"
	"github.com/NilFoundation/nil/msgpool"
)

// SendRawTransaction implements eth_sendRawTransaction. Creates new message or a contract creation for previously-signed message.
func (api *APIImpl) SendRawTransaction(ctx context.Context, encoded hexutil.Bytes) (common.Hash, error) {
	var msg types.Message
	if err := msg.UnmarshalSSZ([]byte(encoded)); err != nil {
		return common.Hash{}, fmt.Errorf("failed to decode message: %w", err)
	}

	shardId := msg.From.ShardId()
	if err := api.checkShard(shardId); err != nil {
		return common.Hash{}, err
	}

	if features.EnableSignatureCheck && !crypto.TransactionSignatureIsValidBytes(msg.Signature[:]) {
		return common.Hash{}, errors.New("invalid signature")
	}

	tx, err := api.db.CreateRwTx(ctx)
	if err != nil {
		return common.Hash{}, err
	}
	defer tx.Rollback()

	reason, err := api.msgPools[shardId].Add(ctx, []*types.Message{&msg}, tx)
	if err != nil {
		return common.Hash{}, err
	}

	err = tx.Commit()
	api.logger.Error().Err(err).Stringer("hash", msg.Hash()).Msg("Failed to commit tx")

	if reason[0] != msgpool.NotSet {
		return common.Hash{}, fmt.Errorf("message status: %s", reason[0])
	}

	return msg.Hash(), nil
}
