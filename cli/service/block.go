package service

import (
	"encoding/json"
)

// FetchBlock fetches the block by number or hash
func (s *Service) FetchBlock(blockId string) ([]byte, error) {
	blockData, err := s.client.GetBlock(s.shardId, blockId, true)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch block")
		return nil, err
	}

	// Marshal the block data into a pretty-printed JSON format
	blockDataJSON, err := json.MarshalIndent(blockData, "", "  ")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal block data to JSON")
		return nil, err
	}

	s.logger.Trace().Msgf("Fetched block:\n%s", blockDataJSON)
	return blockDataJSON, nil
}
