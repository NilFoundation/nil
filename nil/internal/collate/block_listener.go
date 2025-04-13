package collate

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	"google.golang.org/protobuf/proto"
)

const requestTimeout = 10 * time.Second

func topicShardBlocks(shardId types.ShardId) string {
	return fmt.Sprintf("/shard/%s/blocks", shardId)
}

func protocolShardBlock(shardId types.ShardId) network.ProtocolID {
	return network.ProtocolID(fmt.Sprintf("/shard/%s/block", shardId))
}

// ListPeers returns a list of peers that may support block exchange protocol.
func ListPeers(networkManager network.Manager, shardId types.ShardId) []network.PeerID {
	// Try to get peers supporting the protocol.
	if res := networkManager.GetPeersForProtocol(protocolShardBlock(shardId)); len(res) > 0 {
		return res
	}
	// Otherwise, return all peers to try them out.
	return networkManager.AllKnownPeers()
}

// PublishBlock publishes a block to the network.
func PublishBlock(
	ctx context.Context, networkManager network.Manager, shardId types.ShardId, block *types.BlockWithExtractedData,
) error {
	if networkManager == nil {
		// we don't always want to run the network
		return nil
	}

	pbBlock, err := marshalBlockSSZ(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}
	data, err := proto.Marshal(pbBlock)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}
	return networkManager.PubSub().Publish(ctx, topicShardBlocks(shardId), data)
}

func logError(logger logging.Logger, err error, msg string) {
	if err == nil || errors.Is(err, io.EOF) {
		return
	}
	logger.Debug().Err(err).Msg(msg)
}

// Protocol for reading/writing blocks is pretty simple and straightforward:
// 1. Write block size as 8 bytes (big-endian).
// 2. Write block data (in protobuf format).
// That's actually "Length-Delimited Messages".
func readBlockFromStream(s network.Stream) (*types.BlockWithExtractedData, error) {
	header := make([]byte, 8)
	if _, err := io.ReadFull(s, header); err != nil {
		return nil, fmt.Errorf("failed to read block size: %w", err)
	}

	length := binary.BigEndian.Uint64(header)
	buf := make([]byte, length)
	if _, err := io.ReadFull(s, buf); err != nil {
		return nil, fmt.Errorf("failed to read block: %w", err)
	}

	var pbBlock pb.RawFullBlock
	if err := proto.Unmarshal(buf, &pbBlock); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return unmarshalBlockSSZ(&pbBlock)
}

func writeBlockToStream(s network.Stream, block *pb.RawFullBlock) error {
	data, err := proto.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block to Protobuf: %w", err)
	}

	header := make([]byte, 8)
	binary.BigEndian.PutUint64(header, uint64(len(data)))

	if _, err := s.Write(header); err != nil {
		return fmt.Errorf("failed to write block size to stream: %w", err)
	}
	if _, err := s.Write(data); err != nil {
		return fmt.Errorf("failed to write block to stream: %w", err)
	}
	return nil
}

func RequestBlocks(ctx context.Context, networkManager network.Manager, peerID network.PeerID,
	shardId types.ShardId, blockNumber types.BlockNumber, logger logging.Logger,
) (<-chan *types.BlockWithExtractedData, error) {
	var err error
	req, err := proto.Marshal(&pb.BlocksRangeRequest{Id: int64(blockNumber)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal blocks request: %w", err)
	}

	stream, err := networkManager.NewStream(ctx, peerID, protocolShardBlock(shardId))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			stream.Close()
		}
	}()
	if err = stream.SetDeadline(time.Now().Add(requestTimeout)); err != nil {
		return nil, err
	}
	if _, err = stream.Write(req); err != nil {
		return nil, err
	}
	if err = stream.CloseWrite(); err != nil {
		return nil, err
	}

	ch := make(chan *types.BlockWithExtractedData)
	go func() {
		defer stream.Close()
		defer close(ch)

		for {
			block, err := readBlockFromStream(stream)
			if err != nil {
				logError(logger, err, "Failed to handle input block")
				break
			}
			select {
			case ch <- block:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func marshalBlockSSZ(block *types.BlockWithExtractedData) (*pb.RawFullBlock, error) {
	raw, err := block.EncodeSSZ()
	if err != nil {
		return nil, err
	}
	pbBlock := &pb.RawFullBlock{}
	if err := pbBlock.PackProtoMessage(raw); err != nil {
		return nil, err
	}
	return pbBlock, nil
}

func unmarshalBlockSSZ(pbBlock *pb.RawFullBlock) (*types.BlockWithExtractedData, error) {
	raw, err := pbBlock.UnpackProtoMessage()
	if err != nil {
		return nil, err
	}
	return raw.DecodeSSZ()
}

func SetRequestHandler(
	ctx context.Context, networkManager network.Manager, shardId types.ShardId, database db.DB, logger logging.Logger,
) {
	if networkManager == nil {
		// we don't always want to run the network
		return
	}

	// Sharing accessor between all handlers enables caching.
	accessor := execution.NewStateAccessor()
	handler := func(s network.Stream) {
		if err := s.SetDeadline(time.Now().Add(requestTimeout)); err != nil {
			return
		}

		req, err := io.ReadAll(s)
		if err != nil {
			logError(logger, err, "Failed to read request")
			return
		}
		if err = s.CloseRead(); err != nil {
			logError(logger, err, "Failed to close stream for reading")
		}

		var blockReq pb.BlocksRangeRequest
		if err := proto.Unmarshal(req, &blockReq); err != nil {
			logError(logger, err, "Failed to unmarshal block request")
			return
		}

		tx, err := database.CreateRoTx(ctx)
		if err != nil {
			logError(logger, err, "Failed to create transaction")
			return
		}
		defer tx.Rollback()

		acc := accessor.RawAccess(tx, shardId).
			GetBlock().
			WithOutTransactions().
			WithInTransactions().
			WithChildBlocks().
			WithConfig()

		for id := blockReq.Id; ; id++ {
			resp, err := acc.ByNumber(types.BlockNumber(id))
			if err != nil {
				if !errors.Is(err, db.ErrKeyNotFound) {
					logError(logger, err, "DB error")
				}
				break
			}

			b := &pb.RawFullBlock{
				BlockSSZ:           resp.Block(),
				OutTransactionsSSZ: resp.OutTransactions(),
				InTransactionsSSZ:  resp.InTransactions(),
				ChildBlocks:        pb.PackHashes(resp.ChildBlocks()),
				Config:             resp.Config(),
			}

			if err := writeBlockToStream(s, b); err != nil {
				logError(logger, err, "Failed to handle output block")
				return
			}
		}
	}

	networkManager.SetStreamHandler(ctx, protocolShardBlock(shardId), handler)
}
