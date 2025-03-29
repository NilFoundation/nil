package jsonrpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/txnpool"
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

	headers, ok := ctx.Value(transport.HeadersContextKey).(http.Header)
	if !ok {
		headers = http.Header{}
		headers.Add("Client-Type", "failed to extract headers from context")
	}

	log := api.clientEventsLog.Log().
		Stringer(logging.FieldShardId, shardId).
		Str(logging.FieldRpcMethod, "eth_sendRawTransaction").
		Str(logging.FieldClientType, headers.Get("Client-Type")).
		Str(logging.FieldClientVersion, headers.Get("Client-Version")).
		Str(logging.FieldUid, headers.Get("X-UID")) //nolint:canonicalheader

	if err != nil {
		log.Msg("finished with err")
		return common.EmptyHash, err
	}

	if reason != txnpool.NotSet {
		log.Err(ErrTransactionDiscarded).Msgf("%s", reason)
		return common.EmptyHash, fmt.Errorf("%w: %s", ErrTransactionDiscarded, reason)
	}

	h := extTxn.Hash()
	log.Stringer(logging.FieldTransactionHash, h).Msg("added to the pool")
	return h, nil
}
