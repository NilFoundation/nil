package collate

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate/pb"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/multiformats/go-multistream"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
)

type syncCollator struct {
	// todo: remove
	msgPool msgpool.Pool

	shard types.ShardId
	topic string

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
	shard types.ShardId, timeout time.Duration,
	db db.DB, networkManager *network.Manager, bootstrapPeer string,
) (*syncCollator, error) {
	s := &syncCollator{
		msgPool:         msgPool,
		shard:           shard,
		topic:           topicShardBlocks(shard),
		timeout:         timeout,
		db:              db,
		networkManager:  networkManager,
		bootstrapPeer:   bootstrapPeer,
		logger:          logging.NewLogger("sync").With().Stringer(logging.FieldShardId, shard).Logger(),
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
	lastBlock, lastBlockHash, err := db.ReadLastBlock(rotx, s.shard)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}
	if err == nil {
		s.lastBlockNumber = lastBlock.Id
		s.lastBlockHash = lastBlockHash
	}
	return nil
}

func (s *syncCollator) snapIsRequired(ctx context.Context) bool {
	rotx, err := s.db.CreateRoTx(ctx)
	check.PanicIfErr(err)
	defer rotx.Rollback()

	_, _, err = db.ReadLastBlock(rotx, s.shard)
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

	sub, err := s.networkManager.PubSub().Subscribe(s.topic)
	if err != nil {
		return err
	}
	defer sub.Close()

	ch := sub.Start(ctx)
	for {
		select {
		case <-ctx.Done():
			s.logger.Debug().Msg("Sync collator is terminated")
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
			s.logger.Debug().Msgf("No new block in the topic for %s, pulling blocks actively", s.timeout)

			s.fetchBlocks(ctx)
		}
	}
}

func (s *syncCollator) processTopicMessage(ctx context.Context, data []byte) (bool, error) {
	var pbBlock pb.Block
	if err := proto.Unmarshal(data, &pbBlock); err != nil {
		return false, err
	}
	b, err := unmarshalBlockSSZ(&pbBlock)
	if err != nil {
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

		// todo: queue the block for later processing
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

	if err := s.saveBlocks(ctx, []*Block{b}); err != nil {
		return false, err
	}

	return true, nil
}

func (s *syncCollator) fetchBlocks(ctx context.Context) {
	// todo: fetch blocks until the queue (see todo above) is empty
	for {
		s.logger.Debug().Msg("Fetching next block")

		blocks, err := s.fetchBlocksRange(ctx)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to fetch block")
			return
		}
		if len(blocks) == 0 {
			s.logger.Debug().Msg("No new blocks to fetch")
			return
		}
		if err := s.saveBlocks(ctx, blocks); err != nil {
			s.logger.Error().Err(err).Msg("Failed to save blocks")
			return
		}
	}
}

func (s *syncCollator) fetchBlocksRange(ctx context.Context) ([]*Block, error) {
	peers := ListPeers(s.networkManager, s.shard)

	if len(peers) == 0 {
		s.logger.Warn().Msg("No peers to fetch block from")
		return nil, nil
	}

	s.logger.Debug().Msgf("Found %d peers to fetch block from:\n%v", len(peers), peers)

	for _, p := range peers {
		s.logger.Debug().Msgf("Requesting block %d from peer %s", s.lastBlockNumber+1, p)

		const count = 100
		blocks, err := RequestBlocks(ctx, s.networkManager, p, s.shard, s.lastBlockNumber+1, count)
		if err == nil {
			return blocks, nil
		}

		if errors.As(err, &multistream.ErrNotSupported[network.ProtocolID]{}) {
			s.logger.Debug().Err(err).Msgf("Peer %s does not support the block protocol with our shard", p)
		} else {
			s.logger.Warn().Err(err).Msgf("Failed to request block from peer %s", p)
		}
	}

	return nil, nil
}

func (s *syncCollator) saveBlocks(ctx context.Context, blocks []*Block) error {
	if len(blocks) == 0 {
		return nil
	}

	tx, err := s.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var blockHash common.Hash
	var block *Block
	for _, block = range blocks {
		blockHash = block.Block.Hash()
		if err := db.WriteBlock(tx, s.shard, blockHash, block.Block); err != nil {
			return err
		}

		msgRoot, err := s.saveMessages(tx, block.OutMessages)
		if err != nil {
			return err
		}

		if msgRoot != block.Block.OutMessagesRoot {
			return errors.New("out messages root mismatch")
		}

		_, err = execution.PostprocessBlock(tx, s.shard, block.Block.GasPrice, 1, blockHash)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.lastBlockNumber = block.Block.Id
	s.lastBlockHash = blockHash

	s.logger.Debug().
		Stringer(logging.FieldBlockNumber, s.lastBlockNumber).
		Msg("Blocks written")

	return nil
}

func (s *syncCollator) saveMessages(tx db.RwTx, messages []*types.Message) (common.Hash, error) {
	messageTree := execution.NewDbMessageTrie(tx, s.shard)
	indexes := make([]types.MessageIndex, len(messages))
	for i := range messages {
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
