package contracts

import (
	"context"
	"errors"
	"math/big"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
)

type Factory struct {
	Contract
}

func NewFactory(contract Contract) *Factory {
	return &Factory{Contract: contract}
}

func (f *Factory) Deploy(
	service *cliservice.Service,
	deploySmartAccount SmartAccount,
	smartAccount types.Address,
) error {
	argsPacked, err := f.Abi.Pack("", smartAccount)
	if err != nil {
		return err
	}
	code := append(f.Contract.Code.Clone(), argsPacked...)
	f.Addr, err = DeployContract(service, deploySmartAccount.Addr, code)
	return err
}

func (f *Factory) CreatePair(
	ctx context.Context,
	service *cliservice.Service,
	client client.Client,
	smartAccount SmartAccount,
	token0Address types.Address,
	token1Address types.Address,
) error {
	calldata, err := f.Abi.Pack(
		"createPair", token0Address, token1Address, big.NewInt(0), big.NewInt(int64(f.Addr.ShardId())))
	if err != nil {
		return err
	}
	hash, err := client.SendTransactionViaSmartAccount(ctx, smartAccount.Addr, calldata,
		types.FeePack{}, types.NewZeroValue(), []types.TokenBalance{},
		f.Addr,
		smartAccount.PrivateKey)
	if err != nil {
		return err
	}
	_, err = service.WaitForReceiptCommitted(hash)
	if err != nil {
		return err
	}
	return nil
}

func (f *Factory) GetPair(
	service *cliservice.Service,
	token0Address types.Address,
	token1Address types.Address,
) (types.Address, error) {
	res, err := GetFromContract(service, f.Abi, f.Addr, "getTokenPair", token0Address, token1Address)
	if err != nil {
		return types.EmptyAddress, err
	}
	addr, ok := res[0].(types.Address)
	if !ok {
		return types.EmptyAddress, errors.New("failed to unpack token pair address")
	}
	return addr, nil
}
