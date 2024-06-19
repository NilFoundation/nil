package service

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/ethereum/go-ethereum/accounts/abi"
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
func (s *Service) RunContract(wallet types.Address, bytecode []byte, contract types.Address) (string, error) {
	txHash, err := s.client.SendMessageViaWallet(wallet, types.Code(bytecode), contract, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return "", err
	}
	s.logger.Info().Msgf("Transaction hash: %s (shard %s)", txHash, wallet.ShardId())
	return txHash.Hex(), nil
}

// DeployContract deploys a new smart contract with the given bytecode
func (s *Service) DeployContract(shardId types.ShardId, wallet types.Address, bytecode []byte) (string, string, error) {
	txHash, contractAddr, err := s.client.DeployContract(shardId, wallet, bytecode, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return "", "", err
	}
	s.logger.Info().Msgf("Contract address: 0x%x", contractAddr)
	s.logger.Info().Msgf("Transaction hash: %s (shard %s)", txHash, wallet.ShardId())
	return txHash.Hex(), contractAddr.Hex(), nil
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

func parseCallArguments(args []string) []interface{} {
	var parsedArgs []interface{}
	for _, arg := range args {
		if i, ok := new(big.Int).SetString(arg, 10); ok {
			parsedArgs = append(parsedArgs, i)
		} else {
			parsedArgs = append(parsedArgs, arg)
		}
	}
	return parsedArgs
}

func (s *Service) ArgsToCalldata(abiPath string, method string, args []string) ([]byte, error) {
	abiFile, err := os.ReadFile(abiPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ABI file: %w", err)
	}
	var contractAbi abi.ABI
	if err := json.Unmarshal(abiFile, &contractAbi); err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	methodArgs := parseCallArguments(args)
	calldata, err := contractAbi.Pack(method, methodArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack method call: %w", err)
	}
	return calldata, nil
}
