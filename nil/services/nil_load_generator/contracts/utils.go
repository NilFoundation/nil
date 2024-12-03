package contracts

import (
	"crypto/ecdsa"
	"strings"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/ethereum/go-ethereum/crypto"
)

type Wallet struct {
	Addr       types.Address
	PrivateKey *ecdsa.PrivateKey
}

type Contract struct {
	Abi  abi.ABI
	Code types.Code
	Addr types.Address
}

func NewWallet(service *cliservice.Service, shardId types.ShardId) (Wallet, error) {
	pk, err := crypto.GenerateKey()
	if err != nil {
		return Wallet{}, err
	}
	salt := types.NewUint256(0)

	walletAdr, err := service.CreateWallet(shardId, salt, types.GasToValue(1_000_000_000), types.NewValueFromUint64(0), &pk.PublicKey)
	if err != nil {
		if !strings.Contains(err.Error(), "wallet already exists") {
			return Wallet{}, err
		}
		walletCode := contracts.PrepareDefaultWalletForOwnerCode(crypto.CompressPubkey(&pk.PublicKey))
		walletAdr = service.ContractAddress(shardId, *salt, walletCode)
	}
	return Wallet{Addr: walletAdr, PrivateKey: pk}, nil
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

func SendMessageAndCheck(client client.Client, service *cliservice.Service, wallet Wallet, contract types.Address, calldata types.Code, currencies []types.CurrencyBalance) error {
	hash, err := client.SendMessageViaWallet(wallet.Addr, calldata,
		types.NewZeroValue(), types.NewZeroValue(), currencies, contract, wallet.PrivateKey)
	if err != nil {
		return err
	}
	_, err = service.WaitForReceiptCommitted(hash)
	if err != nil {
		return err
	}
	return nil
}

func DeployContract(service *cliservice.Service, wallet types.Address, code types.Code) (types.Address, error) {
	txHashCaller, addr, err := service.DeployContractViaWallet(wallet.ShardId(), wallet, types.BuildDeployPayload(code, wallet.Hash()), types.Value{})
	if err != nil {
		return types.EmptyAddress, err
	}
	_, err = service.WaitForReceiptCommitted(txHashCaller)
	if err != nil {
		return types.EmptyAddress, err
	}
	return addr, nil
}

func TopUpBalance(client client.Client, services []*cliservice.Service, wallets []Wallet, currencies []*Currency) error {
	const balanceThresholdAmount = uint64(1_000_000_000)
	for i, currency := range currencies {
		if err := ensureBalance(services[i/2], currency.Addr, balanceThresholdAmount); err != nil {
			return err
		}
	}

	for i, wallet := range wallets {
		if err := ensureBalance(services[i], wallet.Addr, balanceThresholdAmount); err != nil {
			return err
		}
		if err := ensureWalletCurrencies(client, services[i], wallet, currencies); err != nil {
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

func ensureWalletCurrencies(client client.Client, service *cliservice.Service, wallet Wallet, currencies []*Currency) error {
	const mintThresholdAmount = 100000
	walletCurrency, err := service.GetCurrencies(wallet.Addr)
	if err != nil {
		return err
	}

	for _, currency := range currencies {
		value, ok := walletCurrency[currency.Id]
		if !ok || value.Cmp(types.NewValueFromUint64(mintThresholdAmount)) < 0 {
			if err := currency.MintAndSend(client, service, wallet.Addr, mintThresholdAmount); err != nil {
				return err
			}
		}
	}
	return nil
}
