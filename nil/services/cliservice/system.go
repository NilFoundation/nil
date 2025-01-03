package cliservice

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/types"
)

// Not using generic printer argument here because zerolog events need to be created,
// so it would be printer factory function, which is quite ugly
func ShardsToString(list []types.ShardId) string {
	var str string
	for _, id := range list {
		str += fmt.Sprintf("  * %d\n", id)
	}
	return str
}

func (s *Service) GetShards() ([]types.ShardId, error) {
	list, err := s.client.GetShardIdList(s.ctx)
	if err != nil {
		return nil, err
	}

	s.logger.Info().Msg("List of shard id:")
	s.logger.Info().Msg(ShardsToString(list))
	return list, nil
}

func (s *Service) GetGasPrice(shardId types.ShardId) (types.Value, error) {
	value, err := s.client.GasPrice(s.ctx, shardId)
	if err != nil {
		return types.Value{}, err
	}

	s.logger.Info().Msgf("Gas price of shard %d: %s", shardId, value)
	return value, nil
}

func (s *Service) GetChainId() (types.ChainId, error) {
	value, err := s.client.ChainId(s.ctx)
	if err != nil {
		return types.ChainId(0), err
	}

	s.logger.Info().Msgf("ChainId: %d", value)
	return value, nil
}
