package collate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/rs/zerolog"
)

type syncCollator struct {
	// todo: remove
	msgPool msgpool.Pool
	client  client.Client

	shard types.ShardId

	// Syncer will actively pull blocks if no new blocks appear in the topic for this duration.
	timeout time.Duration

	db             db.DB
	networkManager *network.Manager
	bootstrapPeer  string

	logger zerolog.Logger

	lastBlockNumber types.BlockNumber
	lastBlockHash   common.Hash
}

func NewSyncCollator(ctx context.Context, msgPool msgpool.Pool,
	shard types.ShardId, endpoint string, timeout time.Duration,
	db db.DB, networkManager *network.Manager, bootstrapPeer string,
) (*syncCollator, error) {
	logger := logging.NewLogger("sync").With().Stringer(logging.FieldShardId, shard).Logger()
	s := &syncCollator{
		msgPool:         msgPool,
		client:          rpc_client.NewClient(endpoint, logger),
		shard:           shard,
		timeout:         timeout,
		db:              db,
		networkManager:  networkManager,
		bootstrapPeer:   bootstrapPeer,
		logger:          logger,
		lastBlockNumber: types.BlockNumber(math.MaxUint64),
	}

	return s, nil
}

func (s *syncCollator) readLastBlockNumber(ctx context.Context) error {
	rotx, err := s.db.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer rotx.Rollback()
	lastBlock, err := db.ReadLastBlock(rotx, s.shard)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}
	if err == nil {
		s.lastBlockNumber = lastBlock.Id
		s.lastBlockHash = lastBlock.Hash()
	}
	return nil
}

func (s *syncCollator) snapIsRequired(ctx context.Context) bool {
	rotx, err := s.db.CreateRoTx(ctx)
	check.PanicIfErr(err)
	defer rotx.Rollback()

	_, err = db.ReadLastBlock(rotx, s.shard)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		check.PanicIfErr(err)
	}
	return err != nil
}

func (s *syncCollator) Run(ctx context.Context) error {
	if s.snapIsRequired(ctx) {
		if err := FetchSnapshot(ctx, s.networkManager, s.bootstrapPeer, s.shard, s.db); err != nil {
			return fmt.Errorf("failed to fetch snapshot: %w", err)
		}
	}

	err := s.readLastBlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to read last block number: %w", err)
	}

	s.logger.Debug().
		Stringer(logging.FieldBlockHash, s.lastBlockHash).
		Uint64(logging.FieldBlockNumber, s.lastBlockNumber.Uint64()).
		Msgf("Initialized sync collator at starting block")

	s.logger.Info().Msg("Starting sync")

	var ch <-chan []byte

	if s.networkManager == nil {
		// todo: network must be on
		// make dummy channel for now
		c := make(chan []byte)
		defer close(c)
		ch = c
	} else {
		sub, err := s.networkManager.PubSub().Subscribe(topicShardBlocks(s.shard))
		if err != nil {
			return err
		}
		defer sub.Close()

		ch = sub.Start(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info().Msg("Sync collator is terminated")
			return nil
		case data := <-ch:
			saved, err := s.processTopicMessage(ctx, data)
			if err != nil {
				s.logger.Error().Err(err).Msg("Failed to process topic message")
			}
			if !saved {
				s.fetchBlocks(ctx)
			}
		case <-time.After(s.timeout):
			s.fetchBlocks(ctx)
		}
	}
}

func (s *syncCollator) processTopicMessage(ctx context.Context, data []byte) (bool, error) {
	b := &Block{}
	if err := json.Unmarshal(data, b); err != nil {
		return false, err
	}

	block := b.Block
	s.logger.Debug().
		Stringer(logging.FieldBlockNumber, block.Id).
		Stringer(logging.FieldBlockHash, block.Hash()).
		Msg("Received block")

	if block.Id != s.lastBlockNumber+1 {
		s.logger.Debug().
			Stringer(logging.FieldBlockNumber, block.Id).
			Msgf("Received block is out of order with the last block %d", s.lastBlockNumber)
		return false, nil
	}

	if block.PrevBlock != s.lastBlockHash {
		msg := fmt.Sprintf("Prev block hash mismatch: expected %x, got %x", s.lastBlockHash, block.PrevBlock)
		s.logger.Error().
			Stringer(logging.FieldBlockNumber, block.Id).
			Stringer(logging.FieldBlockHash, block.Hash()).
			Msg(msg)
		panic(msg)
	}

	if err := s.saveBlock(ctx, b); err != nil {
		return false, err
	}

	return true, nil
}

func (s *syncCollator) fetchBlocks(ctx context.Context) {
	for {
		s.logger.Debug().Msg("Fetching next block")
		block, err := s.client.GetDebugBlock(s.shard, uint64(s.lastBlockNumber+1), true)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to fetch block")
			return
		}
		if block == nil {
			s.logger.Debug().Msg("No new block")
			return
		}
		if err := s.saveDebugBlock(ctx, block); err != nil {
			s.logger.Error().Err(err).Msg("Failed to save block")
			return
		}
	}
}

func (s *syncCollator) saveDebugBlock(ctx context.Context, block *jsonrpc.HexedDebugRPCBlock) error {
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
	if data.Block.Id != s.lastBlockNumber+1 {
		s.logger.Error().Msgf(
			"Block number mismatch: expected %d, got %d", s.lastBlockNumber+1, data.Block.Id)
		panic("block number mismatch")
	}

	return s.saveBlock(ctx, &Block{
		Block:       data.Block,
		OutMessages: data.OutMessages,
	})
}

func (s *syncCollator) saveBlock(ctx context.Context, block *Block) error {
	tx, err := s.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := db.WriteBlock(tx, s.shard, block.Block); err != nil {
		return err
	}

	msgRoot, err := s.saveMessages(tx, block.OutMessages)
	if err != nil {
		return err
	}

	if msgRoot != block.Block.OutMessagesRoot {
		return errors.New("out messages root mismatch")
	}

	blockHash := block.Block.Hash()
	_, err = execution.PostprocessBlock(tx, s.shard, block.Block.GasPrice, 1, blockHash)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.lastBlockNumber = block.Block.Id
	s.lastBlockHash = blockHash

	s.logger.Debug().
		Stringer(logging.FieldBlockNumber, s.lastBlockNumber).
		Msg("Block written")

	return nil
}

func (s *syncCollator) saveMessages(tx db.RwTx, messages []*types.Message) (common.Hash, error) {
	messageTree := execution.NewDbMessageTrie(tx, s.shard)
	indexes := make([]types.MessageIndex, len(messages))
	for i := range len(messages) {
		indexes[i] = types.MessageIndex(i)
	}
	if err := messageTree.UpdateBatch(indexes, messages); err != nil {
		return common.EmptyHash, err
	}
	return messageTree.RootHash(), nil
}

func (s *syncCollator) GetMsgPool() msgpool.Pool {
	return s.msgPool
}
