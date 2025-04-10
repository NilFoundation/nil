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
	code, err := s.client.GetCode(s.ctx, contractAddress, "latest")
	if err != nil {
		s.logger.Error().Err(err).Str(logging.FieldRpcMethod, rpc.Eth_getCode).Msg("Failed to get contract code")
		return "", err
	}

	s.logger.Info().Msgf("Contract code: %x", code)
	return code.Hex(), nil
}

// GetBalance retrieves the contract balance at the given address
func (s *Service) GetBalance(contractAddress types.Address) (types.Value, error) {
	balance, err := s.client.GetBalance(s.ctx, contractAddress, "latest")
	if err != nil {
		s.logger.Error().Err(err).Str(logging.FieldRpcMethod, rpc.Eth_getBalance).Msg("Failed to get contract balance")
		return types.Value{}, err
	}

	s.logger.Info().Msgf("Contract balance: %s", balance)
	return balance, nil
}

// GetSeqno retrieves the contract balance at the given address
func (s *Service) GetSeqno(contractAddress types.Address) (types.Seqno, error) {
	seqno, err := s.client.GetTransactionCount(s.ctx, contractAddress, "latest")
	if err != nil {
		s.logger.Error().
			Err(err).
			Str(logging.FieldRpcMethod, rpc.Eth_getTransactionCount).
			Msg("Failed to get contract seqno")
		return types.Seqno(0), err
	}

	s.logger.Info().Msgf("Contract seqno: %d", seqno)
	return seqno, nil
}

// GetInfo returns smart account's address and public key
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

// GetTokens retrieves the contract tokens at the given address
func (s *Service) GetTokens(contractAddress types.Address) (types.TokensMap, error) {
	tokens, err := s.client.GetTokens(s.ctx, contractAddress, "latest")
	if err != nil {
		s.logger.Error().Err(err).Str(logging.FieldRpcMethod, rpc.Eth_getTokens).Msg("Failed to get contract tokens")
		return nil, err
	}

	s.logger.Info().Msg("Contract tokens:")
	for k, v := range tokens {
		s.logger.Info().Stringer(logging.FieldTokenId, k).Msgf("Balance: %v", v)
	}
	return tokens, nil
}

func (s *Service) GetDebugContract(contractAddress types.Address, blockId any) (*jsonrpc.DebugRPCContract, error) {
	contract, err := s.client.GetDebugContract(s.ctx, contractAddress, blockId)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get contract debug information")
		return nil, err
	}

	return contract, nil
}

// RunContract runs bytecode on the specified contract address
func (s *Service) RunContract(smartAccount types.Address, bytecode []byte, fee types.FeePack, value types.Value,
	tokens []types.TokenBalance, contract types.Address,
) (common.Hash, error) {
	txHash, err := s.client.SendTransactionViaSmartAccount(
		s.ctx, smartAccount, bytecode, fee, value, tokens, contract, s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return common.EmptyHash, err
	}
	s.logger.Info().
		Stringer(logging.FieldShardId, smartAccount.ShardId()).
		Stringer(logging.FieldTransactionHash, txHash).
		Send()
	return txHash, nil
}

// SendExternalTransaction runs bytecode on the specified contract address
func (s *Service) SendExternalTransaction(bytecode []byte, contract types.Address, noSign bool) (common.Hash, error) {
	pk := s.privateKey
	if noSign {
		pk = nil
	}
	txHash, err := s.client.SendExternalTransaction(
		s.ctx, types.Code(bytecode), contract, pk, types.NewFeePackFromGas(0))
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send external transaction")
		return common.EmptyHash, err
	}
	s.logger.Info().
		Stringer(logging.FieldShardId, contract.ShardId()).
		Stringer(logging.FieldTransactionHash, txHash).
		Send()
	return txHash, nil
}

// DeployContractViaSmartAccount deploys a new smart contract with the given bytecode via the smart account
func (s *Service) DeployContractViaSmartAccount(
	shardId types.ShardId,
	smartAccount types.Address,
	deployPayload types.DeployPayload,
	value types.Value,
) (common.Hash, types.Address, error) {
	txHash, contractAddr, err := s.client.DeployContract(s.ctx, shardId, smartAccount, deployPayload, value,
		types.NewFeePackFromGas(10_000_000), s.privateKey)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return common.EmptyHash, types.EmptyAddress, err
	}
	s.logger.Info().Msgf("Contract address: 0x%x", contractAddr)
	s.logger.Info().
		Stringer(logging.FieldShardId, shardId).
		Stringer(logging.FieldTransactionHash, txHash).
		Send()
	return txHash, contractAddr, nil
}

// DeployContractExternal deploys a new smart contract with the given bytecode via external transaction
func (s *Service) DeployContractExternal(
	shardId types.ShardId,
	payload types.DeployPayload,
	fee types.FeePack,
) (common.Hash, types.Address, error) {
	txHash, contractAddr, err := s.client.DeployExternal(s.ctx, shardId, payload, fee)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return common.EmptyHash, types.EmptyAddress, err
	}
	s.logger.Info().Msgf("Contract address: 0x%x", contractAddr)
	s.logger.Info().
		Stringer(logging.FieldShardId, shardId).
		Stringer(logging.FieldTransactionHash, txHash).
		Send()
	return txHash, contractAddr, nil
}

// CallContract performs read-only call to the contract
func (s *Service) CallContract(
	contract types.Address, fee types.FeePack, calldata []byte, overrides *jsonrpc.StateOverrides,
) (*jsonrpc.CallRes, error) {
	callArgs := &jsonrpc.CallArgs{
		Data: (*hexutil.Bytes)(&calldata),
		To:   contract,
		Fee:  fee,
	}

	res, err := s.client.Call(s.ctx, callArgs, "latest", overrides)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// EstimateFee returns recommended fee for the call
func (s *Service) EstimateFee(
	contract types.Address, calldata []byte, flags types.TransactionFlags, value types.Value,
) (*jsonrpc.EstimateFeeRes, error) {
	callArgs := &jsonrpc.CallArgs{
		Flags: flags,
		To:    contract,
		Value: value,
		Data:  (*hexutil.Bytes)(&calldata),
	}

	res, err := s.client.EstimateFee(s.ctx, callArgs, "latest")
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Service) ContractAddress(shardId types.ShardId, salt types.Uint256, bytecode []byte) types.Address {
	deployPayload := types.BuildDeployPayload(bytecode, common.Hash(salt.Bytes32()))
	return types.CreateAddress(shardId, deployPayload)
}
