package collate

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/collate/pb"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"google.golang.org/protobuf/proto"
)

type Block struct {
	Block       *types.Block
	OutMessages []*types.Message
	InMessages  []*types.Message
	ShardHashes map[types.ShardId]common.Hash
}

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
func PublishBlock(ctx context.Context, networkManager *network.Manager, shardId types.ShardId, block *Block) error {
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
) ([]*Block, error) {
	req, err := proto.Marshal(&pb.BlockRequest{Id: int64(blockNumber), Count: uint32(count)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal blocks request: %w", err)
	}

	resp, err := networkManager.SendRequestAndGetResponse(ctx, peerID, protocolShardBlock(shardId), req)
	if err != nil {
		return nil, fmt.Errorf("failed to request blocks: %w", err)
	}

	var pbBlocks pb.Blocks
	if err := proto.Unmarshal(resp, &pbBlocks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal blocks: %w", err)
	}
	return unmarshalBlocksSSZ(&pbBlocks)
}

func getBlocksRange(
	ctx context.Context, shardId types.ShardId, accessor *execution.StateAccessor, database db.DB, startId types.BlockNumber, count uint8,
) (*pb.Blocks, error) {
	tx, err := database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res := &pb.Blocks{
		Blocks: make([]*pb.Block, 0, count),
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

		b := &pb.Block{
			BlockSSZ:       resp.Block(),
			OutMessagesSSZ: resp.OutMessages(),
			InMessagesSSZ:  resp.InMessages(),
			ShardHashes:    make(map[uint32][]byte, len(resp.ChildBlocks())),
		}
		for i, child := range resp.ChildBlocks() {
			b.ShardHashes[uint32(i)+1] = child.Bytes()
		}
		res.Blocks = append(res.Blocks, b)
	}

	return res, nil
}

func marshalBlockSSZ(block *Block) (*pb.Block, error) {
	blockSSZ, err := block.Block.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	outMsgs, err := ssz.EncodeContainer[*types.Message](block.OutMessages)
	if err != nil {
		return nil, err
	}
	inMsgs, err := ssz.EncodeContainer[*types.Message](block.InMessages)
	if err != nil {
		return nil, err
	}

	return &pb.Block{
		BlockSSZ:       blockSSZ,
		OutMessagesSSZ: outMsgs,
		InMessagesSSZ:  inMsgs,
		ShardHashes: common.TransformMap(block.ShardHashes, func(key types.ShardId, value common.Hash) (uint32, []byte) {
			return uint32(key), value.Bytes()
		}),
	}, nil
}

func unmarshalBlockSSZ(pbBlock *pb.Block) (*Block, error) {
	block := &types.Block{}
	if err := block.UnmarshalSSZ(pbBlock.BlockSSZ); err != nil {
		return nil, err
	}

	outMsgs, err := ssz.DecodeContainer[*types.Message](pbBlock.OutMessagesSSZ)
	if err != nil {
		return nil, err
	}
	inMsgs, err := ssz.DecodeContainer[*types.Message](pbBlock.InMessagesSSZ)
	if err != nil {
		return nil, err
	}

	return &Block{
		Block:       block,
		OutMessages: outMsgs,
		InMessages:  inMsgs,
		ShardHashes: common.TransformMap(pbBlock.ShardHashes, func(key uint32, value []byte) (types.ShardId, common.Hash) {
			return types.ShardId(key), common.BytesToHash(value)
		}),
	}, nil
}

func unmarshalBlocksSSZ(pbBlocks *pb.Blocks) ([]*Block, error) {
	blocks := make([]*Block, len(pbBlocks.Blocks))
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
		var blockReq pb.BlockRequest
		if err := proto.Unmarshal(req, &blockReq); err != nil {
			return nil, fmt.Errorf("failed to unmarshal block request: %w", err)
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
