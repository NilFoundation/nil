package block

import (
	"encoding/json"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
)

const (
	getBlockByNumber = "eth_getBlockByNumber"
	getBlockByHash   = "eth_getBlockByHash"
)

var logger = common.NewLogger("blockService")

type Service struct {
	client  client.Client
	shardId types.ShardId
}

// NewService initializes a new Service with the given client
func NewService(c client.Client, shardId types.ShardId) *Service {
	return &Service{
		client:  c,
		shardId: shardId,
	}
}

// FetchBlockByNumber fetches the block by number
func (s *Service) FetchBlockByNumber(blockNumber string) ([]byte, error) {
	return s.fetchBlock(getBlockByNumber, blockNumber)
}

// FetchBlockByHash fetches the block by hash
func (s *Service) FetchBlockByHash(blockHash string) ([]byte, error) {
	return s.fetchBlock(getBlockByHash, blockHash)
}

// fetchBlock is a common method to fetch block data based on the method and identifier
func (s *Service) fetchBlock(method, identifier string) ([]byte, error) {
	// Create params for the RPC call
	params := []interface{}{
		s.shardId,
		identifier,
		true,
	}

	// Call the RPC method to fetch the block data
	result, err := s.client.Call(method, params)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to fetch block using method %s", method)
		return nil, err
	}

	// Unmarshal the result into a map
	var blockData map[string]interface{}
	if err := json.Unmarshal(result, &blockData); err != nil {
		logger.Error().Err(err).Msg("Failed to unmarshal block data")
		return nil, err
	}

	// Marshal the block data into a pretty-printed JSON format
	blockDataJSON, err := json.MarshalIndent(blockData, "", "  ")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal block data to JSON")
		return nil, err
	}

	logger.Info().Msgf("Fetched block:\n%s", blockDataJSON)
	return blockDataJSON, nil
}
