package main

import (
	"flag"

	"sync"

	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/shardchain"
)

func main() {
	// parse args
	nshards := flag.Int("nshards", 5, "number of shardchains")

	flag.Parse()

	// each shard will interact with DB via this client
	dbClient := db.NewDBClient()
	shards := make([]*shardchain.ShardChain, 0)
	for i := 0; i < *nshards; i++ {
		shards = append(shards, shardchain.NewShardChain(i, dbClient))
	}

	numClusterTicks := 2
	for t := 0; t < numClusterTicks; t++ {
		var wg sync.WaitGroup

		for i := 0; i < *nshards; i++ {
			wg.Add(1)
			go shards[i].Collate(&wg)
		}

		wg.Wait()
	}
}
