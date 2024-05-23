package jsonrpc

import (
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	ssz "github.com/ferranbt/fastssz"
	"github.com/stretchr/testify/require"
)

func writeTestBlock(t *testing.T, tx db.Tx, shardId types.ShardId, blockNumber types.BlockNumber, messages []*types.Message, receipts []*types.Receipt) common.Hash {
	t.Helper()
	block := types.Block{
		Id:                  blockNumber,
		PrevBlock:           common.EmptyHash,
		SmartContractsRoot:  common.EmptyHash,
		MessagesRoot:        writeMessages(t, tx, shardId, messages).RootHash(),
		ReceiptsRoot:        common.EmptyHash,
		ChildBlocksRootHash: common.EmptyHash,
		MasterChainHash:     common.EmptyHash,
	}
	require.NoError(t, db.WriteBlock(tx, types.MasterShardId, &block))
	return block.Hash()
}

func writeMessages(t *testing.T, tx db.Tx, shardId types.ShardId, messages []*types.Message) *mpt.MerklePatriciaTrie {
	t.Helper()
	messageRoot := mpt.NewMerklePatriciaTrie(tx, shardId, db.MessageTrieTable)
	for i, message := range messages {
		messageBytes, err := message.MarshalSSZ()
		require.NoError(t, err)
		require.NoError(t, messageRoot.Set(ssz.MarshalUint64(nil, uint64(i)), messageBytes))
	}
	return messageRoot
}
