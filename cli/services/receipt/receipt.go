package receipt

import (
	"encoding/json"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
)

const (
	getReceiptByHash = "eth_getInMessageReceipt"
)

var logger = common.NewLogger("receiptService")

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

// FetchReceiptByHash fetches the receipt by hash
func (s *Service) FetchReceiptByHash(receiptHash string) ([]byte, error) {
	return s.fetchReceipt(getReceiptByHash, receiptHash)
}

// fetchReceipt is a common method to fetch receipt data based on the method and identifier
func (s *Service) fetchReceipt(method, identifier string) ([]byte, error) {
	// Create params for the RPC call
	params := []interface{}{
		s.shardId,
		identifier,
	}

	// Call the RPC method to fetch the receipt data
	result, err := s.client.Call(method, params)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to fetch receipt using method %s", method)
		return nil, err
	}

	// Unmarshal the result into a map
	var receiptData map[string]interface{}
	if err := json.Unmarshal(result, &receiptData); err != nil {
		logger.Error().Err(err).Msg("Failed to unmarshal receipt data")
		return nil, err
	}

	// Marshal the receipt data into a pretty-printed JSON format
	receiptDataJSON, err := json.MarshalIndent(receiptData, "", "  ")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal receipt data to JSON")
		return nil, err
	}

	logger.Info().Msgf("Fetched receipt:\n%s", receiptDataJSON)

	return receiptDataJSON, nil
}
