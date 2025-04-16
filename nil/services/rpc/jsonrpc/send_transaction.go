package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/txnpool"
	"github.com/rs/zerolog/log"
)

// SendRawTransaction implements eth_sendRawTransaction.
// Creates new transaction or a contract creation for previously-signed transaction.
func (api *APIImpl) SendRawTransaction(ctx context.Context, encoded hexutil.Bytes) (common.Hash, error) {
	var extTxn types.ExternalTransaction
	if err := extTxn.UnmarshalSSZ(encoded); err != nil {
		return common.EmptyHash, fmt.Errorf("failed to decode transaction: %w", err)
	}

	shardId := extTxn.To.ShardId()
	reason, err := api.rawapi.SendTransaction(ctx, shardId, encoded)
	if err != nil {
		return common.EmptyHash, err
	}

	if reason != txnpool.NotSet {
		log.Err(ErrTransactionDiscarded).Msgf("%s", reason)
		return common.EmptyHash, fmt.Errorf("%w: %s", ErrTransactionDiscarded, reason)
	}

	return extTxn.Hash(), nil
}
