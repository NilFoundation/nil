package shardchain

import (
	"fmt"
	"sync"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/vm"
)

type Transaction struct {
	//address  common.Address
	calldata []byte
}

type ShardChain struct {
	Id   int
	db   db.DB
	pool []Transaction
}

func (c *ShardChain) Collate(wg *sync.WaitGroup) {
	defer wg.Done()

	logger := common.NewLogger(fmt.Sprintf("shard-%d", c.Id), false /* noColor */)
	logger.Info().Msg("running shardchain")

	//tree := db.GetMerkleTree(fmt.Sprintf("shard-%d-smart-contracts", c.Id), c.dbClient)
	// genesisBlock := &types.Block{SmartContracts: tree}

	// nextBlk := genBlock(c.dbClient, genesisBlock, "contract-addr-to-update")
	// genBlock(c.dbClient, nextBlk, "contract-addr-to-update")

	evm := vm.NewEVMInterpreter(nil)
	for _, tx := range c.pool {
		if _, err := evm.Run(&vm.Contract{}, tx.calldata, false); err != nil {
			logger.Error().Msg("transaction failed")
		}
	}
}

func NewShardChain(
	shardId int,
	db db.DB,
) *ShardChain {
	return &ShardChain{shardId, db, nil}
}
