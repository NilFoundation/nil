package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

// TopUpViaFaucet tops up the balance of the contractAddressTo using the faucet contract.
func (api *APIImpl) TopUpViaFaucet(ctx context.Context, faucetAddress, contractAddressTo types.Address, amount types.Value) (common.Hash, error) {
	contractName := contracts.NameFaucet
	if faucetAddress != types.FaucetAddress {
		contractName = contracts.NameFaucetCurrency
	}
	callData, err := contracts.NewCallData(contractName, "withdrawTo", contractAddressTo, amount.ToBig())
	if err != nil {
		return common.EmptyHash, err
	}
	seqno, err := api.GetTransactionCount(ctx, faucetAddress, transport.BlockNumberOrHash(transport.PendingBlock))
	if err != nil {
		return common.EmptyHash, err
	}
	extMsg := &types.ExternalMessage{
		To:        faucetAddress,
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
