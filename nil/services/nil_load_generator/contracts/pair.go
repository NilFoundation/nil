package contracts

import (
	"context"
	"errors"
	"math/big"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
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

func (p *Pair) Initialize(ctx context.Context, service *cliservice.Service, client client.Client, smartAccount SmartAccount, token0, token1 types.Address) error {
	calldata, err := p.Abi.Pack("initialize", token0, token1)
	if err != nil {
		return err
	}
	if _, err := SendTransactionAndCheck(ctx, client, service, smartAccount, p.Addr, calldata, []types.TokenBalance{}); err != nil {
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

func (p *Pair) GetTokenTotalSupply(service *cliservice.Service) (*big.Int, error) {
	res, err := GetFromContract(service, p.Abi, p.Addr, "getTokenTotalSupply")
	if err != nil {
		return nil, err
	}
	totalSupply, ok := res[0].(*big.Int)
	if !ok {
		return nil, errors.New("failed to unpack total supply")
	}
	return totalSupply, nil
}

func (p *Pair) GetTokenBalanceOf(service *cliservice.Service, addr types.Address) (*big.Int, error) {
	res, err := GetFromContract(service, p.Abi, p.Addr, "getTokenBalanceOf", addr)
	if err != nil {
		return nil, err
	}
	totalSupply, ok := res[0].(*big.Int)
	if !ok {
		return nil, errors.New("failed to unpack total supply")
	}
	return totalSupply, nil
}

func (p *Pair) Mint(ctx context.Context, service *cliservice.Service, client client.Client, smartAccount SmartAccount, addressTo types.Address, tokens []types.TokenBalance) error {
	calldata, err := p.Abi.Pack("mint", addressTo)
	if err != nil {
		return err
	}
	if _, err := SendTransactionAndCheck(ctx, client, service, smartAccount, p.Addr, calldata, tokens); err != nil {
		return err
	}
	return nil
}

func (p *Pair) Swap(ctx context.Context, service *cliservice.Service, client client.Client, smartAccount SmartAccount, smartAccountTo types.Address, inputAmount, outputAmount *big.Int, swapAmount types.Value, tokenId types.TokenId) (common.Hash, error) {
	calldata, err := p.Abi.Pack("swap", inputAmount, outputAmount, smartAccountTo)
	if err != nil {
		return common.EmptyHash, err
	}
	hash, err := SendTransactionAndCheck(ctx, client, service, smartAccount, p.Addr, calldata, []types.TokenBalance{
		{Token: tokenId, Balance: swapAmount},
	})
	if err != nil {
		return common.EmptyHash, err
	}
	return hash, nil
}

func (p *Pair) Burn(ctx context.Context, service *cliservice.Service, client client.Client, smartAccount SmartAccount, smartAccountTo types.Address, lpAddress types.TokenId, burnAmount types.Value) error {
	calldata, err := p.Abi.Pack("burn", smartAccountTo)
	if err != nil {
		return err
	}
	if _, err := SendTransactionAndCheck(ctx, client, service, smartAccount, p.Addr, calldata, []types.TokenBalance{
		{Token: lpAddress, Balance: burnAmount},
	}); err != nil {
		return err
	}
	return nil
}
