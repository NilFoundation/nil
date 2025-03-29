package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

// GetBalance implements eth_getBalance. Returns the balance of an account for a given address.
func (api *APIImplRo) GetBalance(
	ctx context.Context,
	address types.Address,
	blockNrOrHash transport.BlockNumberOrHash,
) (*hexutil.Big, error) {
	balance, err := api.rawapi.GetBalance(ctx, address, toBlockReference(blockNrOrHash))
	if err != nil {
		return nil, err
	}
	return hexutil.NewBig(balance.ToBig()), nil
}

// GetTokens implements eth_getTokens. Returns the balance of all tokens of account for a given address.
func (api *APIImplRo) GetTokens(
	ctx context.Context,
	address types.Address,
	blockNrOrHash transport.BlockNumberOrHash,
) (map[types.TokenId]types.Value, error) {
	return api.rawapi.GetTokens(ctx, address, toBlockReference(blockNrOrHash))
}

// GetTransactionCount implements eth_getTransactionCount.
// Returns the number of transactions sent from an address (the nonce / seqno).
func (api *APIImplRo) GetTransactionCount(
	ctx context.Context,
	address types.Address,
	blockNrOrHash transport.BlockNumberOrHash,
) (hexutil.Uint64, error) {
	value, err := api.rawapi.GetTransactionCount(ctx, address, toBlockReference(blockNrOrHash))
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(value), nil
}

// GetCode implements eth_getCode. Returns the byte code at a given address (if it's a smart contract).
func (api *APIImplRo) GetCode(
	ctx context.Context,
	address types.Address,
	blockNrOrHash transport.BlockNumberOrHash,
) (hexutil.Bytes, error) {
	code, err := api.rawapi.GetCode(ctx, address, toBlockReference(blockNrOrHash))
	if err != nil {
		return nil, err
	}
	return hexutil.Bytes(code), nil
}

func blockNrToBlockReference(num transport.BlockNumber) rawapitypes.BlockReference {
	var ref rawapitypes.BlockReference
	if num <= 0 {
		ref = rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.NamedBlockIdentifier(num))
	} else {
		ref = rawapitypes.BlockNumberAsBlockReference(types.BlockNumber(num))
	}
	return ref
}

func toBlockReference(blockNrOrHash transport.BlockNumberOrHash) rawapitypes.BlockReference {
	if number, ok := blockNrOrHash.Number(); ok {
		return blockNrToBlockReference(number)
	}
	hash, ok := blockNrOrHash.Hash()
	check.PanicIfNot(ok)
	return rawapitypes.BlockHashAsBlockReference(hash)
}
