package main

import (
	"flag"
	"log"

	"sync"

	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/shardchain"
)

func main() {
	// parse args
	nshards := flag.Int("nshards", 5, "number of shardchains")

	flag.Parse()

	// each shard will interact with DB via this client
	db, err := db.NewSqlite("test.db")
	if err != nil {
		log.Fatal(err)
	}
	shards := make([]*shardchain.ShardChain, 0)
	for i := 0; i < *nshards; i++ {
		shards = append(shards, shardchain.NewShardChain(i, db))
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
