package cliservice

import (
	"encoding/json"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// FetchReceiptByHash fetches the message receipt by hash
func (s *Service) FetchReceiptByHash(shardId types.ShardId, hash common.Hash) ([]byte, error) {
	receiptData, err := s.client.GetInMessageReceipt(shardId, hash)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch receipt")
		return nil, err
	}

	receiptDataJSON, err := json.MarshalIndent(receiptData, "", "  ")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal receipt data to JSON")
		return nil, err
	}

	s.logger.Info().Msgf("Fetched receipt:\n%s", receiptDataJSON)

	return receiptDataJSON, nil
}
