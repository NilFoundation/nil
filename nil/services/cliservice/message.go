package cliservice

import (
	"encoding/json"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

// FetchMessageByHashJson fetches the message by hash
func (s *Service) FetchMessageByHashJson(shardId types.ShardId, hash common.Hash) ([]byte, error) {
	messageData, err := s.FetchMessageByHash(shardId, hash)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch message")
		return nil, err
	}

	messageDataJSON, err := json.MarshalIndent(messageData, "", "  ")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal message data to JSON")
		return nil, err
	}

	s.logger.Info().Msgf("Fetched message:\n%s", messageDataJSON)
	return messageDataJSON, nil
}

func (s *Service) FetchMessageByHash(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCInMessage, error) {
	return s.client.GetInMessageByHash(shardId, hash)
}
