package contracts

import (
	"context"
	"errors"
	"math/big"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
)

type Pair struct {
	Contract
}

func NewPair(contract Contract, addr types.Address) *Pair {
	return &Pair{
		Contract: Contract{
			Abi:  contract.Abi,
			Code: contract.Code,
			Addr: addr,
		},
	}
}

func (p *Pair) Initialize(ctx context.Context, service *cliservice.Service, client client.Client, wallet Wallet, currency0, currency1 *Currency) error {
	calldata, err := p.Abi.Pack("initialize", currency0.Addr, currency1.Addr, currency0.Id, currency1.Id)
	if err != nil {
		return err
	}
	if err := SendMessageAndCheck(ctx, client, service, wallet, p.Addr, calldata, []types.CurrencyBalance{}); err != nil {
		return err
	}
	return nil
}

func (p *Pair) GetReserves(service *cliservice.Service) (*big.Int, *big.Int, error) {
	res, err := GetFromContract(service, p.Abi, p.Addr, "getReserves")
	if err != nil {
		return nil, nil, err
	}
	first, ok1 := res[0].(*big.Int)
	second, ok2 := res[1].(*big.Int)
	if !ok1 || !ok2 {
		return nil, nil, errors.New("failed to unpack reserves")
	}
	return first, second, nil
}

func (p *Pair) GetCurrencyTotalSupply(service *cliservice.Service) (*big.Int, error) {
	res, err := GetFromContract(service, p.Abi, p.Addr, "getCurrencyTotalSupply")
	if err != nil {
		return nil, err
	}
	totalSupply, ok := res[0].(*big.Int)
	if !ok {
		return nil, errors.New("failed to unpack total supply")
	}
	return totalSupply, nil
}

func (p *Pair) GetCurrencyBalanceOf(service *cliservice.Service, addr types.Address) (*big.Int, error) {
	res, err := GetFromContract(service, p.Abi, p.Addr, "getCurrencyBalanceOf", addr)
	if err != nil {
		return nil, err
	}
	totalSupply, ok := res[0].(*big.Int)
	if !ok {
		return nil, errors.New("failed to unpack total supply")
	}
	return totalSupply, nil
}

func (p *Pair) Mint(ctx context.Context, service *cliservice.Service, client client.Client, wallet Wallet, addressTo types.Address, currencies []types.CurrencyBalance) error {
	calldata, err := p.Abi.Pack("mint", addressTo)
	if err != nil {
		return err
	}
	if err := SendMessageAndCheck(ctx, client, service, wallet, p.Addr, calldata, currencies); err != nil {
		return err
	}
	return nil
}

func (p *Pair) Swap(ctx context.Context, service *cliservice.Service, client client.Client, wallet Wallet, walletTo types.Address, inputAmount, outputAmount *big.Int, swapAmount types.Value, currencyId types.CurrencyId) error {
	calldata, err := p.Abi.Pack("swap", inputAmount, outputAmount, walletTo)
	if err != nil {
		return err
	}
	if err := SendMessageAndCheck(ctx, client, service, wallet, p.Addr, calldata, []types.CurrencyBalance{
		{Currency: currencyId, Balance: swapAmount},
	}); err != nil {
		return err
	}
	return nil
}

func (p *Pair) Burn(ctx context.Context, service *cliservice.Service, client client.Client, wallet Wallet, walletTo types.Address, lpAddress types.CurrencyId, burnAmount types.Value) error {
	calldata, err := p.Abi.Pack("burn", walletTo)
	if err != nil {
		return err
	}
	if err := SendMessageAndCheck(ctx, client, service, wallet, p.Addr, calldata, []types.CurrencyBalance{
		{Currency: lpAddress, Balance: burnAmount},
	}); err != nil {
		return err
	}
	return nil
}
