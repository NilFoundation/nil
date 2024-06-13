package service

import (
	"encoding/json"

	"github.com/NilFoundation/nil/common"
)

// FetchReceiptByHash fetches the receipt by hash
func (s *Service) FetchReceiptByHash(receiptHash string) ([]byte, error) {
	hash := common.BytesToHash([]byte(receiptHash))
	receiptData, err := s.client.GetInMessageReceipt(s.shardId, hash)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch receipt")
		return nil, err
	}

	receiptDataJSON, err := json.MarshalIndent(receiptData, "", "  ")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal receipt data to JSON")
		return nil, err
	}

	s.logger.Trace().Msgf("Fetched receipt:\n%s", receiptDataJSON)

	return receiptDataJSON, nil
}
