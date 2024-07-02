package service

import (
	"github.com/NilFoundation/nil/core/types"
)

func (s *Service) GetShards() ([]types.ShardId, error) {
	list, err := s.client.GetShardIdList()
	if err != nil {
		return nil, err
	}

	s.logger.Info().Msg("List of shard id:")
	for _, id := range list {
		s.logger.Info().Msgf("  * %d", id)
	}
	return list, nil
}

func (s *Service) GetGasPrice(shardId types.ShardId) (types.Value, error) {
	value, err := s.client.GasPrice(shardId)
	if err != nil {
		return types.Value{}, err
	}

	s.logger.Info().Msgf("Gas price of shard %d: %s", shardId, value)
	return value, nil
}

func (s *Service) GetChainId() (types.ChainId, error) {
	value, err := s.client.ChainId()
	if err != nil {
		return types.ChainId(0), err
	}

	s.logger.Info().Msgf("ChainId: %d", value)
	return value, nil
}
