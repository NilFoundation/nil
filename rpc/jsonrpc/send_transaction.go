package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
)

// SendRawTransaction implements eth_sendRawTransaction. Creates new message or a contract creation for previously-signed message.
func (api *APIImpl) SendRawTransaction(ctx context.Context, encoded hexutil.Bytes) (common.Hash, error) {
	var msg types.Message
	if err := msg.UnmarshalSSZ([]byte(encoded)); err != nil {
		return common.Hash{}, fmt.Errorf("failed to decode message: %w", err)
	}

	if common.EnableSignatureCheck && !crypto.TransactionSignatureIsValidBytes(msg.Signature[:]) {
		return common.Hash{}, errors.New("invalid signature")
	}

	tx, err := api.db.CreateRwTx(ctx)
	if err != nil {
		return common.Hash{}, err
	}

	reason, err := api.msgPool.Add(ctx, []*types.Message{&msg}, tx)
	if err != nil {
		return common.Hash{}, err
	}

	if reason[0] != msgpool.NotSet {
		return common.Hash{}, fmt.Errorf("message status: %s", reason[0])
	}

	return msg.Hash(), nil
}
