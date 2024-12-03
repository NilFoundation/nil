package contracts

import (
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

func (f *Factory) Deploy(service *cliservice.Service, deployWallet Wallet, wallet types.Address) error {
	argsPacked, err := f.Abi.Pack("", wallet)
	if err != nil {
		return err
	}
	code := append(f.Contract.Code.Clone(), argsPacked...)
	f.Addr, err = DeployContract(service, deployWallet.Addr, code)
	return err
}

func (f *Factory) CreatePair(service *cliservice.Service, client client.Client, wallet Wallet, currency0Address, currency1Address types.Address) error {
	calldata, err := f.Abi.Pack("createPair", currency0Address, currency1Address, big.NewInt(0), big.NewInt(int64(f.Addr.ShardId())))
	if err != nil {
		return err
	}
	hash, err := client.SendMessageViaWallet(wallet.Addr, calldata,
		types.NewZeroValue(), types.NewZeroValue(), []types.CurrencyBalance{},
		f.Addr,
		wallet.PrivateKey)
	if err != nil {
		return err
	}
	_, err = service.WaitForReceiptCommitted(hash)
	if err != nil {
		return err
	}
	return nil
}

func (f *Factory) GetPair(service *cliservice.Service, currency0Address, currency1Address types.Address) (types.Address, error) {
	res, err := GetFromContract(service, f.Abi, f.Addr, "getTokenPair", currency0Address, currency1Address)
	if err != nil {
		return types.EmptyAddress, err
	}
	addr, ok := res[0].(types.Address)
	if !ok {
		return types.EmptyAddress, errors.New("failed to unpack token pair address")
	}
	return addr, nil
}
