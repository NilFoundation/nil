package main

import (
	"flag"
	"fmt"
	"sync"

	common "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	_ "github.com/NilFoundation/nil/core/db"
	dt "github.com/NilFoundation/nil/core/types"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

func genBlock(updatedAccount string, prevBlock *dt.Block) *dt.Block {
	block := dt.Block{
		Id:             prevBlock.Id + 1,
		PrevBlock:      prevBlock.Hash,
		SmartContracts: prevBlock.SmartContracts,
	}
	// write some info to contract state
	if err := common.UpdateTree(block.SmartContracts.Tree, updatedAccount, fmt.Sprintf("%d", block.Id)); err != nil {
		panic("in a toy cluster merkle tree upd should always succeed :)")
	}

	toHash := common.FilterFieldsByTag(&block, "hashable")
	block.Hash = poseidon.Sum(common.MustSerializeBinaryPersistent(toHash))
	return &block
}

func runShardChain(
	shardId int,
	dbClient *db.DBClient,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	logger := common.NewLogger(fmt.Sprintf("shard-%d", shardId), false /* noColor */)
	logger.Info().Msg("running shardchain")

	tree := common.GetMerkleTree(fmt.Sprintf("shard-%d-smart-contracts", shardId), dbClient)
	genesisBlock := &dt.Block{SmartContracts: tree}

	genBlock("contract-addr", genesisBlock)

	logger.Debug().Msgf("now merkle tree of contracts state is : %+v", tree.Engine)
}

func main() {
	// parse args
	nshards := flag.Int("nshards", 5, "number of shardchains")

	flag.Parse()

	// each shard will interact with DB via this client
	dbClient := db.NewDBClient()

	var wg sync.WaitGroup

	for i := 0; i < *nshards; i++ {
		wg.Add(1)
		go runShardChain(i, dbClient, &wg)
	}

	wg.Wait()
}
