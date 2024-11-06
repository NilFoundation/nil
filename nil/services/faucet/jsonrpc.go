package faucet

import (
	"context"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type FaucetAPI interface {
	TopUpViaFaucet(faucetAddress, contractAddressTo types.Address, amount types.Value) (common.Hash, error)
	GetFaucets() map[string]types.Address
}

type FaucetAPIImpl struct {
	ctx    context.Context
	client client.Client
	logger *zerolog.Logger
}

var _ FaucetAPI = (*FaucetAPIImpl)(nil)

func NewFaucetAPI(ctx context.Context, client client.Client, logger *zerolog.Logger) *FaucetAPIImpl {
	return &FaucetAPIImpl{
		ctx:    ctx,
		client: client,
		logger: logger,
	}
}

func (c *FaucetAPIImpl) TopUpViaFaucet(faucetAddress, contractAddressTo types.Address, amount types.Value) (common.Hash, error) {
	contractName := contracts.NameFaucet
	if faucetAddress != types.FaucetAddress {
		contractName = contracts.NameFaucetCurrency
	}
	callData, err := contracts.NewCallData(contractName, "withdrawTo", contractAddressTo, amount.ToBig())
	if err != nil {
		return common.EmptyHash, err
	}
	seqno, err := c.client.GetTransactionCount(faucetAddress, transport.BlockNumberOrHash(transport.PendingBlock))
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
	return c.client.SendRawTransaction(data)
}

func (c *FaucetAPIImpl) GetFaucets() map[string]types.Address {
	return types.GetCurrencies()
}
