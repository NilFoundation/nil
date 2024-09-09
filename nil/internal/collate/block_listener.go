package collate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type Block struct {
	Block       *types.Block     `json:"block"`
	OutMessages []*types.Message `json:"outMessages,omitempty"`
}

type BlockRequest struct {
	BlockNumber types.BlockNumber `json:"blockNumber"`
	Count       int               `json:"count"`
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

	data, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	return networkManager.PubSub().Publish(ctx, topicShardBlocks(shardId), data)
}

func RequestBlocks(ctx context.Context, networkManager *network.Manager, peerID network.PeerID,
	shardId types.ShardId, blockNumber types.BlockNumber, count int,
) ([]*Block, error) {
	req, err := json.Marshal(&BlockRequest{BlockNumber: blockNumber, Count: count})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal blocks request: %w", err)
	}
	resp, err := networkManager.SendRequestAndGetResponse(ctx, peerID, protocolShardBlock(shardId), req)
	if err != nil {
		return nil, fmt.Errorf("failed to request blocks: %w", err)
	}

	if len(resp) == 0 {
		return nil, nil
	}

	var blocks []*Block
	if err := json.Unmarshal(resp, &blocks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return blocks, nil
}

func SetRequestHandler(ctx context.Context, networkManager *network.Manager, shardId types.ShardId, database db.DB) {
	if networkManager == nil {
		// we don't always want to run the network
		return
	}

	// Sharing accessor between all handlers enables caching.
	accessor, err := execution.NewStateAccessor()
	check.PanicIfErr(err)

	const maxBlockRequestCount = 100
	networkManager.SetRequestHandler(ctx, protocolShardBlock(shardId), func(ctx context.Context, req []byte) ([]byte, error) {
		var blockReq BlockRequest
		if err := json.Unmarshal(req, &blockReq); err != nil {
			return nil, fmt.Errorf("failed to unmarshal block request: %w", err)
		}
		if blockReq.Count <= 0 || maxBlockRequestCount > blockReq.Count {
			return nil, fmt.Errorf("invalid block request count: %d", blockReq.Count)
		}

		tx, err := database.CreateRoTx(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		blocks := make([]*Block, 0, blockReq.Count)
		for i := range blockReq.Count {
			resp, err := accessor.Access(tx, shardId).
				GetBlock().
				WithOutMessages().
				ByNumber(blockReq.BlockNumber + types.BlockNumber(i))
			if err != nil {
				if !errors.Is(err, db.ErrKeyNotFound) {
					return nil, err
				}
				break
			}
			blocks = append(blocks, &Block{Block: resp.Block(), OutMessages: resp.OutMessages()})
		}
		return json.Marshal(blocks)
	})
}
