package collate

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	"google.golang.org/protobuf/proto"
)

func topicShardBlocks(shardId types.ShardId) string {
	return fmt.Sprintf("nil/shard/%s/blocks", shardId)
}

func protocolShardBlock(shardId types.ShardId) network.ProtocolID {
	return network.ProtocolID(fmt.Sprintf("/nil/shard/%s/block", shardId))
}

// ListPeers returns a list of peers that may support block exchange protocol.
func ListPeers(networkManager *network.Manager, shardId types.ShardId) []network.PeerID {
	// Try to get peers supporting the protocol.
	if res := networkManager.GetPeersForProtocol(protocolShardBlock(shardId)); len(res) > 0 {
		return res
	}
	// Otherwise, return all peers to try them out.
	return networkManager.AllKnownPeers()
}

// PublishBlock publishes a block to the network.
func PublishBlock(
	ctx context.Context, networkManager *network.Manager, shardId types.ShardId, block *types.BlockWithExtractedData,
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

func RequestBlocks(ctx context.Context, networkManager *network.Manager, peerID network.PeerID,
	shardId types.ShardId, blockNumber types.BlockNumber, count uint8,
) ([]*types.BlockWithExtractedData, error) {
	req, err := proto.Marshal(&pb.BlocksRangeRequest{Id: int64(blockNumber), Count: uint32(count)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal blocks request: %w", err)
	}

	resp, err := networkManager.SendRequestAndGetResponse(ctx, peerID, protocolShardBlock(shardId), req)
	if err != nil {
		return nil, fmt.Errorf("failed to request blocks: %w", err)
	}

	var pbBlocks pb.RawFullBlocks
	if err := proto.Unmarshal(resp, &pbBlocks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal blocks: %w", err)
	}
	return unmarshalBlocksSSZ(&pbBlocks)
}

func getBlocksRange(
	ctx context.Context, shardId types.ShardId, accessor *execution.StateAccessor, database db.DB, startId types.BlockNumber, count uint8,
) (*pb.RawFullBlocks, error) {
	tx, err := database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res := &pb.RawFullBlocks{
		Blocks: make([]*pb.RawFullBlock, 0, count),
	}
	for i := range count {
		resp, err := accessor.RawAccess(tx, shardId).
			GetBlock().
			WithOutMessages().
			WithInMessages().
			WithChildBlocks().
			ByNumber(startId + types.BlockNumber(i))
		if err != nil {
			if !errors.Is(err, db.ErrKeyNotFound) {
				return nil, err
			}
			break
		}

		b := &pb.RawFullBlock{
			BlockSSZ:       resp.Block(),
			OutMessagesSSZ: resp.OutMessages(),
			InMessagesSSZ:  resp.InMessages(),
			ChildBlocks:    pb.PackHashes(resp.ChildBlocks()),
		}
		res.Blocks = append(res.Blocks, b)
	}

	return res, nil
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

func unmarshalBlocksSSZ(pbBlocks *pb.RawFullBlocks) ([]*types.BlockWithExtractedData, error) {
	blocks := make([]*types.BlockWithExtractedData, len(pbBlocks.Blocks))
	var err error
	for i, pbBlock := range pbBlocks.Blocks {
		blocks[i], err = unmarshalBlockSSZ(pbBlock)
		if err != nil {
			return nil, err
		}
	}
	return blocks, nil
}

func SetRequestHandler(ctx context.Context, networkManager *network.Manager, shardId types.ShardId, database db.DB) {
	if networkManager == nil {
		// we don't always want to run the network
		return
	}

	// Sharing accessor between all handlers enables caching.
	accessor := execution.NewStateAccessor()
	handler := func(ctx context.Context, req []byte) ([]byte, error) {
		var blockReq pb.BlocksRangeRequest
		if err := proto.Unmarshal(req, &blockReq); err != nil {
			return nil, fmt.Errorf("failed to unmarshal block range request: %w", err)
		}

		const maxBlockRequestCount = 100
		if maxBlockRequestCount > blockReq.Count {
			return nil, fmt.Errorf("invalid block request count: %d", blockReq.Count)
		}

		blocks, err := getBlocksRange(
			ctx, shardId, accessor, database, types.BlockNumber(blockReq.Id), uint8(blockReq.Count))
		if err != nil {
			return nil, err
		}

		return proto.Marshal(blocks)
	}

	networkManager.SetRequestHandler(ctx, protocolShardBlock(shardId), handler)
}
