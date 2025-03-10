package signer

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
)

var errBlockVerify = errors.New("failed to verify block")

type BlockVerifier struct {
	shardId types.ShardId
	db      db.DB
}

func NewBlockVerifier(shardId types.ShardId, db db.DB) *BlockVerifier {
	return &BlockVerifier{
		shardId: shardId,
		db:      db,
	}
}

func (b *BlockVerifier) VerifyBlock(ctx context.Context, block *types.Block) error {
	params, err := config.GetConfigParams(ctx, b.db, b.shardId, block.Id.Uint64())
	if err != nil {
		return fmt.Errorf("%w: failed to get validators' params: %w", errBlockVerify, err)
	}

	if err := block.VerifySignature(params.PublicKeys.Keys(), b.shardId); err != nil {
		return fmt.Errorf("%w: failed to verify signature: %w", errBlockVerify, err)
	}
	return nil
}
