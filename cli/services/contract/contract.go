package contract

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
)

const (
	sendRawTransaction  = "eth_sendRawTransaction"
	getCode             = "eth_getCode"
	getTransactionCount = "eth_getTransactionCount"
)

var logger = common.NewLogger("contractService")

type Service struct {
	client     client.Client
	privateKey *ecdsa.PrivateKey
}

// NewService initializes a new Service with the given client and private key
func NewService(client client.Client, pk string) *Service {
	privateKey, err := crypto.HexToECDSA(pk)
	if err != nil {
		logger.Fatal().Msg("Failed to parse private key")
	}

	return &Service{
		client,
		privateKey,
	}
}

// GetCode retrieves the contract code at the given address
func (s *Service) GetCode(contractAddress string) (string, error) {
	// Define the block number (hardcoded to latest block)
	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}

	// Convert the contract address from string to common.Address
	address := types.HexToAddress(contractAddress)

	// Create params for the RPC call
	params := []interface{}{
		address,
		blockNum,
	}

	// Call the RPC method to get the contract code
	result, err := s.client.Call(getCode, params)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to getCode")
		return "", err
	}

	// Unmarshal the result into a string
	var code string
	if err := json.Unmarshal(result, &code); err != nil {
		logger.Error().Err(err).Msg("Failed to unmarshal contract code")
		return "", err
	}

	logger.Info().Msgf("Contract code: %s", code)
	return code, nil
}

// RunContract runs bytecode on the specified contract address
func (s *Service) RunContract(bytecode string, contractAddress string) (string, error) {
	// Get the public key from the private key
	pubKey, ok := s.privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		logger.Fatal().Msgf("Failed to get public key")
		return "", errors.New("failed to get public key")
	}

	// Convert the public key to a public address
	publicAddress := types.PubkeyBytesToAddress(0, crypto.CompressPubkey(pubKey))

	// Get the sequence number for the public address
	seqNum, err := s.getSeqNum(publicAddress.Hex())
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get sequence number")
		return "", err
	}

	// Create the message with the bytecode to run
	message := types.Message{
		From:  publicAddress,
		To:    types.HexToAddress(contractAddress),
		Data:  hexutil.FromHex(bytecode),
		Seqno: seqNum,
	}

	// Sign the message with the private key
	err = message.Sign(s.privateKey)
	if err != nil {
		return "", err
	}

	// Marshal the message to SSZ format
	mData, err := message.MarshalSSZ()
	if err != nil {
		return "", err
	}

	// Send the raw transaction
	txHash, err := s.sendRawTransaction(mData)
	if err != nil {
		return "", err
	}

	return txHash, nil
}

// DeployContract deploys a new smart contract with the given bytecode
func (s *Service) DeployContract(bytecode string) (string, error) {
	// Get the public key from the private key
	pubKey, ok := s.privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		logger.Fatal().Msgf("Could not assert the public key to ecdsa public key")
		return "", errors.New("could not assert the public key to ecdsa public key")
	}

	// Convert the public key to a public address
	publicAddress := types.PubkeyBytesToAddress(0, crypto.CompressPubkey(pubKey))

	// Get the sequence number for the public address
	seqNum, err := s.getSeqNum(publicAddress.Hex())
	if err != nil {
		return "", err
	}

	// Create the deploy message
	dm := &types.DeployMessage{
		ShardId: publicAddress.ShardId(),
		Code:    hexutil.FromHex(bytecode),
		Seqno:   seqNum,
	}
	data, err := dm.MarshalSSZ()
	if err != nil {
		return "", err
	}

	// Create the message with the deploy data
	message := types.Message{
		From:  publicAddress,
		Seqno: seqNum,
		Data:  data,
	}

	// Sign the message with the private key
	err = message.Sign(s.privateKey)
	if err != nil {
		return "", err
	}

	// Marshal the message to SSZ format
	mData, err := message.MarshalSSZ()
	if err != nil {
		return "", err
	}

	// Send the raw transaction
	txHash, err := s.sendRawTransaction(mData)
	if err != nil {
		return "", err
	}

	return txHash, nil
}

// sendRawTransaction sends a raw transaction to the cluster
func (s *Service) sendRawTransaction(messageData []byte) (string, error) {
	// Encode the message data to hex and create the RPC call parameters
	params := []interface{}{"0x" + hex.EncodeToString(messageData)}

	// Call the RPC method to send the raw transaction
	result, err := s.client.Call(sendRawTransaction, params)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to send new transaction")
		return "", err
	}

	// Unmarshal the result into a transaction hash
	var txHash string
	if err := json.Unmarshal(result, &txHash); err != nil {
		logger.Error().Err(err).Msg("Failed to unmarshal transaction hash")
		return "", err
	}

	logger.Info().Msgf("Transaction hash: %s", txHash)
	return txHash, nil
}

// getSeqNum gets the sequence number for the given address
func (s *Service) getSeqNum(address string) (uint64, error) {
	// Define the block number (latest block)
	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}

	// Create params for the RPC call
	params := []interface{}{
		address,
		blockNum,
	}

	// Call the RPC method to get the transaction count
	result, err := s.client.Call(getTransactionCount, params)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to get transaction count for address: %s", address)
		return 0, err
	}

	// Unmarshal the result into a string
	var seqNumStr string
	if err := json.Unmarshal(result, &seqNumStr); err != nil {
		logger.Error().Err(err).Msg("Failed to unmarshal sequence number")
		return 0, err
	}

	logger.Info().Msgf("Sequence number (string): %s", seqNumStr)

	// Convert the hexadecimal string to uint64
	seqNum, err := strconv.ParseUint(seqNumStr[2:], 16, 64)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to convert sequence number to uint64")
		return 0, err
	}

	logger.Info().Msgf("Sequence number (uint64): %d", seqNum)
	return seqNum, nil
}
