package config

import (
	"context"
	"sync"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	lru "github.com/hashicorp/golang-lru/v2"
)

var GlobalConfigCache *ConfigCache

func InitGlobalConfigCache(nShards uint32, txFabric db.DB) error {
	var err error
	GlobalConfigCache, err = NewConfigCache(nShards, txFabric)
	return err
}

func GetConfigParams(ctx context.Context, txFabric db.DB, shardId types.ShardId, height uint64) (*ConfigParams, error) {
	if GlobalConfigCache != nil {
		return GlobalConfigCache.GetParams(ctx, shardId, height)
	}
	value := &cacheValue{
		txFabric: txFabric,
		shardId:  shardId,
		height:   height,
	}
	// SAFETY: value is not shared
	if err := value.initUnsafe(ctx); err != nil {
		return nil, err
	}
	return &value.ConfigParams, nil
}

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

func (c *ConfigCache) GetParams(ctx context.Context, shardId types.ShardId, height uint64) (*ConfigParams, error) {
	if int(shardId) >= len(c.configLru) {
		return nil, types.NewError(types.ErrorShardIdIsTooBig)
	}

	cache := c.configLru[shardId]
	value := &cacheValue{
		txFabric: c.txFabric,
		shardId:  shardId,
		height:   height,
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
}

func mergeValidators(input []ListValidators) []ValidatorInfo {
	var result []ValidatorInfo
	visited := make(map[Pubkey]struct{})

	for _, shardValidators := range input {
		for _, v := range shardValidators.List {
			if _, ok := visited[v.PublicKey]; ok {
				continue
			}
			visited[v.PublicKey] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

func (v *cacheValue) getValidatorsList(configAccessor ConfigAccessor) ([]ValidatorInfo, error) {
	validatorsList, err := getParamImpl[ParamValidators](configAccessor)
	if err != nil {
		return nil, err
	}
	if v.shardId.IsMainShard() {
		return mergeValidators(validatorsList.Validators), nil
	}
	if int(v.shardId)-1 >= len(validatorsList.Validators) {
		return nil, types.NewError(types.ErrorShardIdIsTooBig)
	}
	return validatorsList.Validators[v.shardId-1].List, nil
}

func (v *cacheValue) initUnsafe(ctx context.Context) error {
	tx, err := v.txFabric.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	block, err := db.ReadBlockByNumber(tx, v.shardId, types.BlockNumber(max(v.height, 1)-1))
	if err != nil {
		return err
	}
	var configAccessor ConfigAccessor
	configAccessor, err = NewConfigAccessorFromBlockWithTx(tx, block, v.shardId)
	if err != nil {
		return err
	}
	v.ValidatorInfo, err = v.getValidatorsList(configAccessor)
	if err != nil {
		return err
	}
	v.PublicKeys, err = CreateValidatorsPublicKeyMap(v.ValidatorInfo)
	if err != nil {
		return err
	}
	v.GasPrice, err = GetParamGasPrice(configAccessor)
	if err != nil {
		return err
	}
	v.L1BlockInfo, err = GetParamL1Block(configAccessor)
	return err
}

func (v *cacheValue) init(ctx context.Context) {
	v.once.Do(func() {
		v.err = v.initUnsafe(ctx)
	})
}
