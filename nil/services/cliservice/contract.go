package cliservice

import (
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/crypto"
)

// GetCode retrieves the contract code at the given address
func (s *Service) GetCode(contractAddress types.Address) (string, error) {
	code, err := s.client.GetCode(contractAddress, "latest")
	if err != nil {
		s.logger.Error().Err(err).Str(logging.FieldRpcMethod, rpc.Eth_getCode).Msg("Failed to get contract code")
		return "", err
	}

	s.logger.Info().Msgf("Contract code: %x", code)
	return code.Hex(), nil
}

// GetBalance retrieves the contract balance at the given address
func (s *Service) GetBalance(contractAddress types.Address) (types.Value, error) {
	balance, err := s.client.GetBalance(contractAddress, "latest")
	if err != nil {
		s.logger.Error().Err(err).Str(logging.FieldRpcMethod, rpc.Eth_getBalance).Msg("Failed to get contract balance")
		return types.Value{}, err
	}

	s.logger.Info().Msgf("Contract balance: %s", balance)
	return balance, nil
}

// GetInfo returns wallet's address and public key
func (s *Service) GetInfo(address types.Address) (string, string, error) {
	s.logger.Info().Msgf("Address: %s", address)

	var pub string
	if s.privateKey != nil {
		pubBytes := crypto.CompressPubkey(&s.privateKey.PublicKey)
		pub = hexutil.Encode(pubBytes)
		s.logger.Info().Msgf("Public key: %s", pub)
	}

	return address.String(), pub, nil
}

// GetCurrencies retrieves the contract currencies at the given address
func (s *Service) GetCurrencies(contractAddress types.Address) (types.CurrenciesMap, error) {
	currencies, err := s.client.GetCurrencies(contractAddress, "latest")
	if err != nil {
		s.logger.Error().Err(err).Str(logging.FieldRpcMethod, rpc.Eth_getCurrencies).Msg("Failed to get contract currencies")
		return nil, err
	}

	s.logger.Info().Msg("Contract currencies:")
	for k, v := range currencies {
		s.logger.Info().Str(logging.FieldCurrencyId, k).Msgf("Balance: %v", v)
	}
	return currencies, nil
}

// RunContract runs bytecode on the specified contract address
func (s *Service) RunContract(wallet types.Address, bytecode []byte, feeCredit, value types.Value,
	currencies []types.CurrencyBalance, contract types.Address,
) (common.Hash, error) {
	txHash, err := s.client.SendMessageViaWallet(wallet, bytecode, feeCredit, value, currencies, contract, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return common.EmptyHash, err
	}
	s.logger.Info().
		Stringer(logging.FieldShardId, wallet.ShardId()).
		Stringer(logging.FieldMessageHash, txHash).
		Send()
	return txHash, nil
}

// SendExternalMessage runs bytecode on the specified contract address
func (s *Service) SendExternalMessage(bytecode []byte, contract types.Address, noSign bool) (common.Hash, error) {
	pk := s.privateKey
	if noSign {
		pk = nil
	}
	txHash, err := s.client.SendExternalMessage(types.Code(bytecode), contract, pk, types.Value{})
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send external message")
		return common.EmptyHash, err
	}
	s.logger.Info().
		Stringer(logging.FieldShardId, contract.ShardId()).
		Stringer(logging.FieldMessageHash, txHash).
		Send()
	return txHash, nil
}

// DeployContractViaWallet deploys a new smart contract with the given bytecode via the wallet
func (s *Service) DeployContractViaWallet(shardId types.ShardId, wallet types.Address, deployPayload types.DeployPayload, value types.Value) (common.Hash, types.Address, error) {
	txHash, contractAddr, err := s.client.DeployContract(shardId, wallet, deployPayload, value, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return common.EmptyHash, types.EmptyAddress, err
	}
	s.logger.Info().Msgf("Contract address: 0x%x", contractAddr)
	s.logger.Info().
		Stringer(logging.FieldShardId, shardId).
		Stringer(logging.FieldMessageHash, txHash).
		Send()
	return txHash, contractAddr, nil
}

// DeployContractExternal deploys a new smart contract with the given bytecode via external message
func (s *Service) DeployContractExternal(shardId types.ShardId, payload types.DeployPayload, feeCredit types.Value) (common.Hash, types.Address, error) {
	txHash, contractAddr, err := s.client.DeployExternal(shardId, payload, feeCredit)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return common.EmptyHash, types.EmptyAddress, err
	}
	s.logger.Info().Msgf("Contract address: 0x%x", contractAddr)
	s.logger.Info().
		Stringer(logging.FieldShardId, shardId).
		Stringer(logging.FieldMessageHash, txHash).
		Send()
	return txHash, contractAddr, nil
}

// CallContract performs read-only call to the contract
func (s *Service) CallContract(
	contract types.Address, feeCredit types.Value, calldata []byte, overrides *jsonrpc.StateOverrides,
) (*jsonrpc.CallRes, error) {
	callArgs := &jsonrpc.CallArgs{
		Data:      (*hexutil.Bytes)(&calldata),
		To:        contract,
		FeeCredit: feeCredit,
	}

	res, err := s.client.Call(callArgs, "latest", overrides)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// EstimateFee returns recommended fee for the call
func (s *Service) EstimateFee(
	contract types.Address, calldata []byte, flags types.MessageFlags, value types.Value,
) (types.Value, error) {
	callArgs := &jsonrpc.CallArgs{
		Flags: flags,
		To:    contract,
		Value: value,
		Data:  (*hexutil.Bytes)(&calldata),
	}

	res, err := s.client.EstimateFee(callArgs, "latest")
	if err != nil {
		return types.Value{}, err
	}
	return res, nil
}

func (s *Service) ContractAddress(shardId types.ShardId, salt types.Uint256, bytecode []byte) types.Address {
	deployPayload := types.BuildDeployPayload(bytecode, common.Hash(salt.Bytes32()))
	return types.CreateAddress(shardId, deployPayload)
}
