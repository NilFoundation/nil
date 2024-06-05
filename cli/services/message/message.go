package message

import (
	"encoding/json"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
)

const (
	getTransactionByHash = "eth_getInMessageByHash"
)

var logger = common.NewLogger("messageService")

type Service struct {
	client client.Client
}

// NewService initializes a new Service with the given client
func NewService(c client.Client) *Service {
	return &Service{
		client: c,
	}
}

// FetchMessageByHash fetches the message by hash
func (s *Service) FetchMessageByHash(messageHash string) ([]byte, error) {
	return s.fetchMessage(getTransactionByHash, messageHash)
}

// fetchMessage is a common method to fetch message data based on the method and identifier
func (s *Service) fetchMessage(method, identifier string) ([]byte, error) {
	// Create params for the RPC call
	params := []interface{}{
		types.BaseShardId,
		identifier,
	}

	// Call the RPC method to fetch the message data
	result, err := s.client.Call(method, params)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to fetch message using method %s", method)
		return nil, err
	}

	// Unmarshal the result into a map
	var messageData map[string]interface{}
	if err := json.Unmarshal(result, &messageData); err != nil {
		logger.Error().Err(err).Msg("Failed to unmarshal message data")
		return nil, err
	}

	// Marshal the message data into a pretty-printed JSON format
	messageDataJSON, err := json.MarshalIndent(messageData, "", "  ")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal message data to JSON")
		return nil, err
	}

	logger.Info().Msgf("Fetched message:\n%s", messageDataJSON)
	return messageDataJSON, nil
}
