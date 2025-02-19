package config

import (
	"context"
	"sync"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog"
)

const lruCacheSize = 16

type ConfigCache struct {
	configLru []*lru.Cache[uint64, *cacheValue]

	txFabric db.DB
}

func NewConfigCache(nShards uint32, txFabric db.DB) (*ConfigCache, error) {
	configLru := make([]*lru.Cache[uint64, *cacheValue], 0, nShards)
	for range nShards {
		cache, err := lru.New[uint64, *cacheValue](lruCacheSize)
		if err != nil {
			return nil, err
		}
		configLru = append(configLru, cache)
	}
	return &ConfigCache{
		configLru: configLru,
		txFabric:  txFabric,
	}, nil
}

func (c *ConfigCache) GetParams(ctx context.Context, shardId types.ShardId, height uint64, logger zerolog.Logger) (*ConfigParams, error) {
	if int(shardId) >= len(c.configLru) {
		return nil, types.NewError(types.ErrorShardIdIsTooBig)
	}

	cache := c.configLru[shardId]
	value := &cacheValue{
		txFabric: c.txFabric,
		shardId:  shardId,
		height:   height,
		logger:   logger,
	}

	// Note:  this is suboptimal, but hashicorp/golang-lru doesn't provide GetOrAdd,
	//		  there is a PR though: https://github.com/hashicorp/golang-lru/pull/170
	cache.ContainsOrAdd(height, value)
	value, ok := cache.Get(height)
	check.PanicIfNot(ok)

	value.init(ctx)
	if value.err != nil {
		// This is likely to happen if we try to get validators for a height that is not yet available.
		// In this case, we should not cache the error, because the error is not permanent.
		cache.Remove(height)
		return nil, value.err
	}
	return &value.ConfigParams, nil
}

type ConfigParams struct {
	ValidatorInfo []ValidatorInfo
	PublicKeys    *PublicKeyMap
	GasPrice      *ParamGasPrice
	L1BlockInfo   *ParamL1BlockInfo
}

type cacheValue struct {
	ConfigParams

	txFabric db.DB

	shardId types.ShardId
	height  uint64

	err error

	once sync.Once

	logger zerolog.Logger
}

func (v *cacheValue) initImpl(ctx context.Context) {
	var configAccessor ConfigAccessor
	configAccessor, v.err = GetConfigAccessorForShard(ctx, v.txFabric, types.BlockNumber(v.height), v.shardId, v.logger)
	if v.err != nil {
		return
	}
	v.ValidatorInfo, v.err = GetValidatorsFromConfigAccessor(configAccessor, v.shardId)
	if v.err != nil {
		return
	}
	v.PublicKeys, v.err = CreateValidatorsPublicKeyMap(v.ValidatorInfo)
	if v.err != nil {
		return
	}
	v.GasPrice, v.err = GetParamGasPrice(configAccessor)
	if v.err != nil {
		return
	}
	v.L1BlockInfo, v.err = GetParamL1Block(configAccessor)
}

func (v *cacheValue) init(ctx context.Context) {
	v.once.Do(func() {
		v.initImpl(ctx)
	})
}
