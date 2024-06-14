package service

import (
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/contracts"
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
	// Get the sequence number for the wallet
	seqnoWallet, err := s.getSeqNum(address)
	if err != nil {
		return "", err
	}

	calldata := hexutil.FromHex(bytecode)
	contract := types.HexToAddress(contractAddress)
	wallet := types.HexToAddress(address)

	intMsg := &types.Message{
		Data:     calldata,
		From:     wallet,
		To:       contract,
		Value:    *types.NewUint256(0),
		GasLimit: *types.NewUint256(100000),
		Internal: true,
	}

	intMsgData, err := intMsg.MarshalSSZ()
	if err != nil {
		return "", err
	}

	walletAbi, err := contracts.GetAbi("Wallet")
	if err != nil {
		return "", err
	}

	calldataExt, err := walletAbi.Pack("send", intMsgData)
	if err != nil {
		return "", err
	}

	// Create the message with the bytecode to run
	extMsg := &types.ExternalMessage{
		To:    wallet,
		Data:  calldataExt,
		Seqno: seqnoWallet,
	}

	// Sign the message with the private key
	err = extMsg.Sign(s.privateKey)
	if err != nil {
		return "", err
	}

	// Send the raw transaction
	txHash, err := s.sendRawTransaction(extMsg)
	if err != nil {
		return "", err
	}

	return txHash, nil
}

// DeployContract deploys a new smart contract with the given bytecode
func (s *Service) DeployContract(address string, bytecode string) (string, string, error) {
	publicAddress := types.HexToAddress(address)
	code := hexutil.FromHex(bytecode)

	// Calculate contract address
	addrWallet := types.CreateAddress(s.shardId, code)

	// Get the sequence number for the public address
	seqno, err := s.getSeqNum(publicAddress.Hex())
	if err != nil {
		return "", "", err
	}

	// Create the message with the deploy data
	intMsg := &types.Message{
		Seqno:    seqno,
		From:     publicAddress,
		To:       addrWallet,
		Value:    *types.NewUint256(0),
		GasLimit: *types.NewUint256(100000),
		Data:     types.BuildDeployPayload(hexutil.FromHex(bytecode), common.EmptyHash).Bytes(),
		Deploy:   true,
		Internal: true,
	}

	msgInternalData, err := intMsg.MarshalSSZ()
	if err != nil {
		return "", "", err
	}

	// Make external message to the address
	walletAbi, err := contracts.GetAbi("Wallet")
	if err != nil {
		return "", "", err
	}

	// Send a message with the deploy data
	calldata, err := walletAbi.Pack("send", msgInternalData)
	if err != nil {
		return "", "", err
	}

	// Create external message to the wallet
	extMsg := &types.ExternalMessage{
		Seqno: seqno,
		To:    publicAddress,
		Data:  calldata,
	}

	if err := extMsg.Sign(s.privateKey); err != nil {
		return "", "", err
	}

	// Send the raw transaction
	txHash, err := s.sendRawTransaction(extMsg)
	if err != nil {
		return "", "", err
	}
	return txHash, addrWallet.Hex(), nil
}

// sendRawTransaction sends a raw transaction to the cluster
func (s *Service) sendRawTransaction(message *types.ExternalMessage) (string, error) {
	// Call the RPC method to send the raw transaction
	txHash, err := s.client.SendMessage(message)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send new transaction")
		return "", err
	}

	s.logger.Trace().Msgf("Transaction hash: %s", txHash)
	return txHash.String(), nil
}

// getSeqNum gets the sequence number for the given address
func (s *Service) getSeqNum(address string) (types.Seqno, error) {
	// Define the block number (the latest block)
	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}

	addr := types.HexToAddress(address)
	seqNum, err := s.client.GetTransactionCount(addr, blockNum)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get sequence number")
		return 0, err
	}

	s.logger.Trace().Msgf("Sequence number (uint64): %d", seqNum)
	return seqNum, nil
}
