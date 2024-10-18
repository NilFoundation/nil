package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

// GetBalance implements eth_getBalance. Returns the balance of an account for a given address.
func (api *APIImplRo) GetBalance(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Big, error) {
	balance, err := api.rawapi.GetBalance(ctx, address, toBlockReference(blockNrOrHash))
	if err != nil {
		return nil, err
	}
	return hexutil.NewBig(balance.ToBig()), nil
}

// GetCurrencies implements eth_getCurrencies. Returns the balance of all currencies of account for a given address.
func (api *APIImplRo) GetCurrencies(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (map[string]*hexutil.Big, error) {
	currencies, err := api.rawapi.GetCurrencies(ctx, address, toBlockReference(blockNrOrHash))
	if err != nil {
		return nil, err
	}
	return common.TransformMap(currencies, func(k types.CurrencyId, v types.Value) (string, *hexutil.Big) {
		return hexutil.ToHexNoLeadingZeroes(k[:]), hexutil.NewBig(v.ToBig())
	}), nil
}

// GetTransactionCount implements eth_getTransactionCount. Returns the number of transactions sent from an address (the nonce / seqno).
func (api *APIImplRo) GetTransactionCount(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Uint64, error) {
	value, err := api.rawapi.GetMessageCount(ctx, address, toBlockReference(blockNrOrHash))
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(value), nil
}

// GetCode implements eth_getCode. Returns the byte code at a given address (if it's a smart contract).
func (api *APIImplRo) GetCode(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Bytes, error) {
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
