package contracts

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"strings"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/ethereum/go-ethereum/crypto"
)

type SmartAccount struct {
	Addr       types.Address
	PrivateKey *ecdsa.PrivateKey
	CliService *cliservice.Service
}

type Contract struct {
	Abi  abi.ABI
	Code types.Code
	Addr types.Address
}

func NewSmartAccount(service *cliservice.Service, shardId types.ShardId) (SmartAccount, error) {
	pk, err := crypto.GenerateKey()
	if err != nil {
		return SmartAccount{}, err
	}
	salt := types.NewUint256(0)

	smartAccountAdr, err := service.CreateSmartAccount(
		shardId, salt, types.GasToValue(1_000_000_000), types.FeePack{}, &pk.PublicKey)
	if err != nil {
		if !strings.Contains(err.Error(), "smart account already exists") {
			return SmartAccount{}, err
		}
		smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(crypto.CompressPubkey(&pk.PublicKey))
		smartAccountAdr = service.ContractAddress(shardId, *salt, smartAccountCode)
	}
	return SmartAccount{Addr: smartAccountAdr, PrivateKey: pk, CliService: service.CloneWithPrivateKey(pk)}, nil
}

func (sa *SmartAccount) ensureBalance(threshold types.Uint256) error {
	balance, err := sa.CliService.GetBalance(sa.Addr)
	if err != nil {
		return err
	}
	if balance.Cmp(types.Value{Uint256: &threshold}) < 0 {
		if err := sa.CliService.TopUpViaFaucet(
			types.FaucetAddress,
			sa.Addr,
			types.Value{Uint256: &threshold},
		); err != nil {
			return err
		}
	}
	return nil
}

func (sa *SmartAccount) SendTransactionAndCheck(
	ctx context.Context,
	contract types.Address,
	calldata types.Code,
	tokens []types.TokenBalance,
) (common.Hash, error) {
	hash, err := sa.CliService.Client().SendTransactionViaSmartAccount(
		ctx,
		sa.Addr,
		calldata,
		types.FeePack{},
		types.NewZeroValue(),
		tokens,
		contract,
		sa.PrivateKey)
	if err != nil {
		return common.EmptyHash, err
	}
	rcp, err := sa.CliService.WaitForReceiptCommitted(hash)
	if err != nil {
		return common.EmptyHash, err
	}
	if !rcp.AllSuccess() {
		return common.EmptyHash, errors.New(rcp.ErrorMessage)
	}
	return hash, nil
}

func GetFromContract(
	service *cliservice.Service,
	abi abi.ABI,
	addr types.Address,
	name string,
	args ...any,
) ([]any, error) {
	calldata, err := abi.Pack(name, args...)
	if err != nil {
		return nil, err
	}
	data, err := service.CallContract(addr, types.NewFeePackFromGas(1_000_000), calldata, nil)
	if err != nil {
		return nil, err
	}
	return abi.Unpack(name, data.Data)
}

func DeployContract(service *cliservice.Service, smartAccount types.Address, code types.Code) (types.Address, error) {
	txHashCaller, addr, err := service.DeployContractViaSmartAccount(smartAccount.ShardId(), smartAccount,
		types.BuildDeployPayload(code, smartAccount.Hash()), types.Value{})
	if err != nil {
		return types.EmptyAddress, err
	}
	_, err = service.WaitForReceiptCommitted(txHashCaller)
	if err != nil {
		return types.EmptyAddress, err
	}
	return addr, nil
}

func TopUpBalance(
	balanceThresholdAmount types.Uint256,
	smartAccounts []SmartAccount,
) error {
	for _, smartAccount := range smartAccounts {
		tkn, err := smartAccount.CliService.GetTokens(smartAccount.Addr)
		if err != nil {
			return err
		}
		topUpTokens := []types.Address{types.EthFaucetAddress, types.UsdcFaucetAddress, types.UsdtFaucetAddress}
		for _, tokenAddr := range topUpTokens {
			v, ok := tkn[*types.TokenIdForAddress(tokenAddr)]
			if !ok || v.Cmp(types.Value{Uint256: &balanceThresholdAmount}) < 0 {
				if err := smartAccount.CliService.TopUpViaFaucet(tokenAddr, smartAccount.Addr, types.Value{
					Uint256: &balanceThresholdAmount,
				}); err != nil {
					return err
				}
			}
		}
		if err := smartAccount.ensureBalance(balanceThresholdAmount); err != nil {
			return err
		}
	}
	return nil
}
