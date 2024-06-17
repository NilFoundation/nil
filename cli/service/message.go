package service

import (
	"encoding/json"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
)

// FetchMessageByHash fetches the message by hash
func (s *Service) FetchMessageByHash(shardId types.ShardId, hash common.Hash) ([]byte, error) {
	messageData, err := s.client.GetInMessageByHash(shardId, hash)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch message")
		return nil, err
	}

	messageDataJSON, err := json.MarshalIndent(messageData, "", "  ")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal message data to JSON")
		return nil, err
	}

	s.logger.Trace().Msgf("Fetched message:\n%s", messageDataJSON)
	return messageDataJSON, nil
}
