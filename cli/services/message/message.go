package message

import (
	"encoding/json"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
)

var logger = logging.NewLogger("messageService")

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

// FetchMessageByHash fetches the message by hash
func (s *Service) FetchMessageByHash(messageHash string) ([]byte, error) {
	hash := common.BytesToHash([]byte(messageHash))
	messageData, err := s.client.GetInMessageByHash(s.shardId, hash)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch message")
		return nil, err
	}

	messageDataJSON, err := json.MarshalIndent(messageData, "", "  ")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal message data to JSON")
		return nil, err
	}

	logger.Trace().Msgf("Fetched message:\n%s", messageDataJSON)
	return messageDataJSON, nil
}
