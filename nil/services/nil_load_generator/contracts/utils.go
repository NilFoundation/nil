package contracts

import (
	"context"
	"crypto/ecdsa"
	"strings"

	"github.com/NilFoundation/nil/nil/client"
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

	smartAccountAdr, err := service.CreateSmartAccount(shardId, salt, types.GasToValue(1_000_000_000), types.NewValueFromUint64(0), &pk.PublicKey)
	if err != nil {
		if !strings.Contains(err.Error(), "smart account already exists") {
			return SmartAccount{}, err
		}
		smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(crypto.CompressPubkey(&pk.PublicKey))
		smartAccountAdr = service.ContractAddress(shardId, *salt, smartAccountCode)
	}
	return SmartAccount{Addr: smartAccountAdr, PrivateKey: pk}, nil
}

func GetFromContract(service *cliservice.Service, abi abi.ABI, addr types.Address, name string, args ...any) ([]any, error) {
	calldata, err := abi.Pack(name, args...)
	if err != nil {
		return nil, err
	}
	data, err := service.CallContract(addr, types.GasToValue(1_000_000), calldata, nil)
	if err != nil {
		return nil, err
	}
	return abi.Unpack(name, data.Data)
}

func SendTransactionAndCheck(ctx context.Context, client client.Client, service *cliservice.Service, smartAccount SmartAccount, contract types.Address, calldata types.Code, tokens []types.TokenBalance) (common.Hash, error) {
	hash, err := client.SendTransactionViaSmartAccount(ctx, smartAccount.Addr, calldata,
		types.NewZeroValue(), types.NewZeroValue(), tokens, contract, smartAccount.PrivateKey)
	if err != nil {
		return common.EmptyHash, err
	}
	_, err = service.WaitForReceiptCommitted(hash)
	if err != nil {
		return common.EmptyHash, err
	}
	return hash, nil
}

func DeployContract(service *cliservice.Service, smartAccount types.Address, code types.Code) (types.Address, error) {
	txHashCaller, addr, err := service.DeployContractViaSmartAccount(smartAccount.ShardId(), smartAccount, types.BuildDeployPayload(code, smartAccount.Hash()), types.Value{})
	if err != nil {
		return types.EmptyAddress, err
	}
	_, err = service.WaitForReceiptCommitted(txHashCaller)
	if err != nil {
		return types.EmptyAddress, err
	}
	return addr, nil
}

func TopUpBalance(services []*cliservice.Service, smartAccounts []SmartAccount) error {
	const balanceThresholdAmount = uint64(1_000_000_000)
	for i, smartAccount := range smartAccounts {
		tkn, err := services[i].GetTokens(smartAccount.Addr)
		if err != nil {
			return err
		}
		topUpTokens := []types.Address{types.EthFaucetAddress, types.UsdcFaucetAddress, types.UsdtFaucetAddress}
		for _, tokenAddr := range topUpTokens {
			if v, ok := tkn[*types.TokenIdForAddress(tokenAddr)]; !ok || v.Cmp(types.NewValueFromUint64(balanceThresholdAmount)) < 0 {
				if err := services[i].TopUpViaFaucet(tokenAddr, smartAccount.Addr, types.NewValueFromUint64(balanceThresholdAmount)); err != nil {
					return err
				}
			}
		}
		if err := ensureBalance(services[i], smartAccount.Addr, balanceThresholdAmount); err != nil {
			return err
		}
	}
	return nil
}

func ensureBalance(service *cliservice.Service, addr types.Address, threshold uint64) error {
	balance, err := service.GetBalance(addr)
	if err != nil {
		return err
	}
	if balance.Uint64() < threshold {
		if err := service.TopUpViaFaucet(types.FaucetAddress, addr, types.NewValueFromUint64(threshold)); err != nil {
			return err
		}
	}
	return nil
}
