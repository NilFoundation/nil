package service

import (
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
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

	s.logger.Info().Msgf("Contract code: %x", code)
	return code.Hex(), nil
}

// RunContract runs bytecode on the specified contract address
func (s *Service) RunContract(wallet types.Address, bytecode string, contract types.Address) (string, error) {
	calldata := hexutil.FromHex(bytecode)

	txHash, err := s.client.SendMessageViaWallet(wallet, types.Code(calldata), contract, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return "", err
	}
	s.logger.Info().Msgf("Transaction hash (shard %s): %s", txHash, wallet.ShardId())
	return txHash.Hex(), nil
}

// DeployContract deploys a new smart contract with the given bytecode
func (s *Service) DeployContract(shardId types.ShardId, wallet types.Address, bytecode string) (string, string, error) {
	code := hexutil.FromHex(bytecode)

	txHash, contractAddr, err := s.client.DeployContract(shardId, wallet, code, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return "", "", err
	}
	s.logger.Info().Msgf("Contract address: 0x%x", contractAddr)
	s.logger.Info().Msgf("Transaction hash: %s (shard %s)", txHash, wallet.ShardId())
	return txHash.Hex(), contractAddr.Hex(), nil
}

// CallContract performs read-only call to the contract
func (s *Service) CallContract(contract types.Address, calldata string) (string, error) {
	data := hexutil.FromHex(calldata)

	seqno := hexutil.Uint64(0)
	callArgs := &jsonrpc.CallArgs{
		From:     contract,
		Data:     data,
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
