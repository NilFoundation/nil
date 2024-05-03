package shardchain

import (
	"fmt"
	"sync"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
)

type ShardChain struct {
	Id int
	db db.DB
}

func (c *ShardChain) Collate(wg *sync.WaitGroup) {
	defer wg.Done()

	logger := common.NewLogger(fmt.Sprintf("shard-%d", c.Id), false /* noColor */)
	logger.Info().Msg("running shardchain")

	//tree := db.GetMerkleTree(fmt.Sprintf("shard-%d-smart-contracts", c.Id), c.dbClient)
	// genesisBlock := &types.Block{SmartContracts: tree}

	// nextBlk := genBlock(c.dbClient, genesisBlock, "contract-addr-to-update")
	// genBlock(c.dbClient, nextBlk, "contract-addr-to-update")

	//logger.Debug().Msgf("now merkle tree of contracts state is : %+v", tree.Engine)
}

func NewShardChain(
	shardId int,
	db db.DB,
) *ShardChain {
	return &ShardChain{shardId, db}
}
