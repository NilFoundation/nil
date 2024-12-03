package contracts

import (
	"crypto/ecdsa"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/ethereum/go-ethereum/crypto"
)

type Currency struct {
	Contract
	Name        string
	OwnerKey    []byte
	OwnerWallet Wallet
	Id          types.CurrencyId
}

func NewCurrency(contract Contract, name string, ownerWallet Wallet) *Currency {
	return &Currency{
		Contract:    contract,
		Name:        name,
		OwnerKey:    crypto.CompressPubkey(&ownerWallet.PrivateKey.PublicKey),
		OwnerWallet: ownerWallet,
		Id:          types.CurrencyId{0},
	}
}

func (c *Currency) Deploy(service *cliservice.Service, deployWallet Wallet) error {
	argsPacked, err := c.Abi.Pack("", c.Name, c.OwnerKey)
	if err != nil {
		return err
	}
	code := append(c.Contract.Code.Clone(), argsPacked...)
	c.Addr, err = DeployContract(service, deployWallet.Addr, code)
	if err != nil {
		return err
	}
	c.Id = types.CurrencyId(c.Addr)
	return nil
}

func (c *Currency) MintAndSend(client client.Client, service *cliservice.Service, walletTo types.Address, mintAmount uint64) error {
	calldata, err := c.Abi.Pack("mintCurrency", types.NewValueFromUint64(mintAmount))
	if err != nil {
		return err
	}
	if err := sendExternalMessage(client, service, calldata, c.Addr, c.OwnerWallet.PrivateKey); err != nil {
		return err
	}
	calldata, err = c.Abi.Pack("sendCurrency", walletTo, c.Id, types.NewValueFromUint64(mintAmount))
	if err != nil {
		return err
	}
	if err := sendExternalMessage(client, service, calldata, c.Addr, c.OwnerWallet.PrivateKey); err != nil {
		return err
	}
	return nil
}

func sendExternalMessage(client client.Client, service *cliservice.Service, calldata types.Code, contractAddr types.Address, pk *ecdsa.PrivateKey) error {
	hash, err := client.SendExternalMessage(calldata, contractAddr, pk, types.NewZeroValue())
	if err != nil {
		return err
	}
	_, err = service.WaitForReceiptCommitted(hash)
	if err != nil {
		return err
	}
	return nil
}
