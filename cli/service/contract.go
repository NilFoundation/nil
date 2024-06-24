package service

import (
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
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
func (s *Service) GetBalance(contractAddress types.Address) (string, error) {
	balance, err := s.client.GetBalance(contractAddress, "latest")
	if err != nil {
		s.logger.Error().Err(err).Str(logging.FieldRpcMethod, rpc.Eth_getCode).Msg("Failed to get contract balance")
		return "", err
	}

	s.logger.Info().Msgf("Contract balance: %s", balance)
	return balance.String(), nil
}

// RunContract runs bytecode on the specified contract address
func (s *Service) RunContract(wallet types.Address, bytecode []byte, value *types.Uint256, contract types.Address) (string, error) {
	txHash, err := s.client.SendMessageViaWallet(wallet, bytecode, types.NewUint256(100_000), value,
		[]types.CurrencyBalance{}, contract, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return "", err
	}
	s.logger.Info().Msgf("Transaction hash: %s (shard %s)", txHash, wallet.ShardId())
	return txHash.Hex(), nil
}

// SendExternalMessage runs bytecode on the specified contract address
func (s *Service) SendExternalMessage(bytecode []byte, contract types.Address, noSign bool) (string, error) {
	pk := s.privateKey
	if noSign {
		pk = nil
	}
	txHash, err := s.client.SendExternalMessage(types.Code(bytecode), contract, pk)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send external message")
		return "", err
	}
	s.logger.Info().Msgf("Transaction hash: %s (shard %s)", txHash, contract.ShardId())
	return txHash.Hex(), nil
}

// DeployContractViaWallet deploys a new smart contract with the given bytecode via the wallet
func (s *Service) DeployContractViaWallet(shardId types.ShardId, wallet types.Address, bytecode []byte, value *types.Uint256) (string, string, error) {
	txHash, contractAddr, err := s.client.DeployContract(shardId, wallet, bytecode, value, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return "", "", err
	}
	s.logger.Info().Msgf("Contract address: 0x%x", contractAddr)
	s.logger.Info().Msgf("Transaction hash: %s (shard %s)", txHash, wallet.ShardId())
	return txHash.Hex(), contractAddr.Hex(), nil
}

// DeployContractExternal deploys a new smart contract with the given bytecode via external message
func (s *Service) DeployContractExternal(shardId types.ShardId, bytecode []byte) (common.Hash, types.Address, error) {
	txHash, contractAddr, err := s.client.DeployExternal(shardId, bytecode)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return common.EmptyHash, types.EmptyAddress, err
	}
	s.logger.Info().Msgf("Contract address: 0x%x", contractAddr)
	s.logger.Info().Msgf("Transaction hash: %s (shard %s)", txHash, contractAddr.ShardId())
	return txHash, contractAddr, nil
}

// CallContract performs read-only call to the contract
func (s *Service) CallContract(contract types.Address, calldata []byte) (string, error) {
	seqno := hexutil.Uint64(0)
	callArgs := &jsonrpc.CallArgs{
		From:     contract,
		Data:     calldata,
		To:       contract,
		Value:    types.NewUint256(0),
		GasLimit: types.NewUint256(10000),
		Seqno:    &seqno,
	}

	res, err := s.client.Call(callArgs)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to call")
		return "", err
	}
	s.logger.Info().Msgf("Call result: %s", res)
	return res, nil
}
