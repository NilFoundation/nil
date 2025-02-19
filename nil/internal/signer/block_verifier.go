package signer

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
)

var errBlockVerify = errors.New("failed to verify block")

type BlockVerifier struct {
	shardId types.ShardId
	cache   *config.ConfigCache
}

func NewBlockVerifier(shardId types.ShardId, cache *config.ConfigCache) *BlockVerifier {
	return &BlockVerifier{
		shardId: shardId,
		cache:   cache,
	}
}

func (b *BlockVerifier) VerifyBlock(ctx context.Context, block *types.Block, logger zerolog.Logger) error {
	params, err := b.cache.GetParams(ctx, b.shardId, block.Id.Uint64(), logger)
	if err != nil {
		return fmt.Errorf("%w: failed to get config params: %w", errBlockVerify, err)
	}

	if err := block.VerifySignature(params.PublicKeys.Keys(), b.shardId); err != nil {
		return fmt.Errorf("%w: failed to verify signature: %w", errBlockVerify, err)
	}
	return nil
}
