package jsonrpc

import (
	"context"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"math/big"
)

// TopUpViaFaucet tops up the balance of the contractAddressTo using the faucet contract.
func (api *APIImpl) TopUpViaFaucet(ctx context.Context, contractAddressFrom, contractAddressTo types.Address, amount types.Value) (common.Hash, error) {
	var callData []byte
	var err error
	if contractAddressFrom == types.FaucetAddress {
		callData, err = contracts.NewCallData(contracts.NameFaucet, "withdrawTo", contractAddressTo, amount.ToBig())
	} else {
		callData, err = contracts.NewCallData(contracts.NameFaucet, "withdrawCurrencyTo", contractAddressFrom, contractAddressTo, new(big.Int).SetBytes(contractAddressFrom.Bytes()), amount.ToBig())
	}
	if err != nil {
		return common.EmptyHash, err
	}
	seqno, err := api.GetTransactionCount(ctx, types.FaucetAddress, transport.BlockNumberOrHash(transport.PendingBlock))
	if err != nil {
		return common.EmptyHash, err
	}
	extMsg := &types.ExternalMessage{
		To:        types.FaucetAddress,
		Data:      callData,
		Seqno:     types.Seqno(seqno.Uint64()),
		Kind:      types.ExecutionMessageKind,
		FeeCredit: types.GasToValue(100_000),
	}
	data, err := extMsg.MarshalSSZ()
	if err != nil {
		return common.EmptyHash, err
	}
	return api.SendRawTransaction(ctx, data)
}
