package collate

import (
	"context"
	"errors"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type syncCollator struct {
	msgPool  msgpool.Pool
	logger   zerolog.Logger
	shard    types.ShardId
	endpoint string
	client   client.Client
	db       db.DB

	lastBlockNumber transport.BlockNumber
	lastBlockHash   common.Hash
}

func NewSyncCollator(ctx context.Context, msgPool msgpool.Pool, shard types.ShardId, endpoint string, db db.DB) (*syncCollator, error) {
	logger := logging.NewLogger("collator").With().Stringer(logging.FieldShardId, shard).Logger()
	s := syncCollator{
		msgPool:  msgPool,
		logger:   logger,
		shard:    shard,
		endpoint: endpoint,
		client:   rpc_client.NewClient(endpoint, logger),
		db:       db,
	}
	err := s.readLastBlockNumber(ctx)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *syncCollator) readLastBlockNumber(ctx context.Context) error {
	rotx, err := s.db.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer rotx.Rollback()
	lastBlock, err := db.ReadLastBlock(rotx, s.shard)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		s.logger.Error().Err(err).Msg("Failed to read last block")
		return err
	}
	if err == nil {
		s.lastBlockNumber = transport.BlockNumber(lastBlock.Id)
		s.lastBlockHash = lastBlock.Hash()
	} else {
		s.lastBlockNumber = transport.BlockNumber(-1)
	}
	s.logger.Info().Stringer("hash", s.lastBlockHash).Uint64("number", s.lastBlockNumber.Uint64()).Msgf("Last block")
	return nil
}

func (s *syncCollator) Run(ctx context.Context) error {
	s.logger.Info().Msg("Starting sync")
	for {
		select {
		case <-ctx.Done():
			s.logger.Info().Msg("Sync collator is terminated")
			return nil
		case <-time.After(2 * time.Second):
			s.fetchBlock(ctx)
		}
	}
}

func (s *syncCollator) fetchBlock(ctx context.Context) {
	for {
		s.logger.Debug().Msg("Fetching next block")
		block, err := s.client.GetDebugBlock(s.shard, s.lastBlockNumber+1, true)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to fetch block")
			return
		}
		if block == nil {
			s.logger.Debug().Msg("No new block")
			return
		}
		if err = s.saveBlock(ctx, block); err != nil {
			s.logger.Error().Err(err).Msg("Failed to save block")
			return
		}
	}
}

func (s *syncCollator) saveBlock(ctx context.Context, block *jsonrpc.HexedDebugRPCBlock) error {
	s.logger.Trace().Msgf("Fetched block: %s", block)
	data, err := block.DecodeHexAndSSZ()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to decode block data from hexed SSZ")
		return err
	}
	if data.Block.PrevBlock != s.lastBlockHash {
		s.logger.Error().Msgf(
			"Prev block hash mismatch: expected %x, got %x", s.lastBlockHash, data.Block.PrevBlock)
		panic("prev block hash mismatch")
	}
	if data.Block.Id != types.BlockNumber(s.lastBlockNumber+1) {
		s.logger.Error().Msgf(
			"Block number mismatch: expected %d, got %d", s.lastBlockNumber+1, data.Block.Id)
		panic("block number mismatch")
	}
	tx, err := s.db.CreateRwTx(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create rw tx")
		return err
	}
	defer tx.Rollback()
	err = db.WriteBlock(tx, s.shard, data.Block)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to write block")
		return err
	}
	msgRoot, err := s.saveMessages(tx, data.OutMessages)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to save messages")
		return err
	}
	if msgRoot != data.Block.OutMessagesRoot {
		s.logger.Error().Msgf("Out messages root mismatch: expected %x, got %x", data.Block.OutMessagesRoot, msgRoot)
		return errors.New("out messages root mismatch")
	}
	blockHash := data.Block.Hash()
	err = db.WriteLastBlockHash(tx, s.shard, blockHash)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to write last block hash")
		return err
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to commit tx")
		return err
	}
	s.lastBlockNumber = transport.BlockNumber(data.Block.Id)
	s.lastBlockHash = blockHash
	s.logger.Info().Msgf("Block %d is written", s.lastBlockNumber)
	return nil
}

func (s *syncCollator) saveMessages(tx db.RwTx, messages []*types.Message) (common.Hash, error) {
	messageTree := execution.NewDbMessageTrie(tx, s.shard)
	for outMsgIndex, msg := range messages {
		if err := messageTree.Update(types.MessageIndex(outMsgIndex), msg); err != nil {
			return common.EmptyHash, err
		}
	}
	return messageTree.RootHash(), nil
}

func (s *syncCollator) GetMsgPool() msgpool.Pool {
	return s.msgPool
}
