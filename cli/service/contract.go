package service

import (
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
)

// GetCode retrieves the contract code at the given address
func (s *Service) GetCode(contractAddress string) (string, error) {
	// Define the block number (hardcoded to latest block)
	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}

	// Convert the contract address from string to common.Address
	address := types.HexToAddress(contractAddress)

	code, err := s.client.GetCode(address, blockNum)
	if err != nil {
		s.logger.Error().Err(err).Str(logging.FieldRpcMethod, rpc.Eth_getCode).Msg("Failed to get contract code")
		return "", err
	}

	s.logger.Trace().Msgf("Contract code: %x", code)
	return code.Hex(), nil
}

// RunContract runs bytecode on the specified contract address
func (s *Service) RunContract(address string, bytecode string, contractAddress string) (string, error) {
	calldata := hexutil.FromHex(bytecode)
	contract := types.HexToAddress(contractAddress)
	wallet := types.HexToAddress(address)

	txHash, err := s.client.SendMessageViaWallet(wallet, types.Code(calldata), contract, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return "", err
	}
	s.logger.Trace().Msgf("Transaction hash: %s", txHash)
	return txHash.Hex(), nil
}

// DeployContract deploys a new smart contract with the given bytecode
func (s *Service) DeployContract(shardId types.ShardId, address string, bytecode string) (string, string, error) {
	wallet := types.HexToAddress(address)
	code := hexutil.FromHex(bytecode)

	txHash, contractAddr, err := s.client.DeployContract(shardId, wallet, code, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return "", "", err
	}
	s.logger.Trace().Msgf("Transaction hash: %s", txHash)
	return txHash.Hex(), contractAddr.Hex(), nil
}
