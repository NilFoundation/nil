package service

import (
	"encoding/json"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
)

// FetchBlockByNumber fetches the block by number
func (s *Service) FetchBlockByNumber(blockNumber string) ([]byte, error) {
	var bn transport.BlockNumber
	if err := bn.UnmarshalJSON([]byte(blockNumber)); err != nil {
		s.logger.Error().Err(err).Msg("Invalid block number")
		return nil, err
	}

	return s.onFetchBlock(s.client.GetBlockByNumber(s.shardId, bn, true))
}

// FetchBlockByHash fetches the block by hash
func (s *Service) FetchBlockByHash(blockHash string) ([]byte, error) {
	var hash common.Hash
	if err := hash.UnmarshalText([]byte(blockHash)); err != nil {
		s.logger.Error().Err(err).Msg("Invalid hash")
		return nil, err
	}

	return s.onFetchBlock(s.client.GetBlockByHash(s.shardId, hash, true))
}

// onFetchBlock is a callback that handles result from server (prints block or error)
func (s *Service) onFetchBlock(blockData *jsonrpc.RPCBlock, err error) ([]byte, error) {
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
