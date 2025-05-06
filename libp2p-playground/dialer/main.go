package main

import (
	"context"
	"log"
	"os"
	"time"

	libp2plog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/redis/go-redis/v9"
)

func main() {
	libp2plog.SetDebugLogging()

	ctx := context.Background()

	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/4005"))
	if err != nil {
		log.Fatal(err)
	}

	lai, err := getRelayedListenerAddr(ctx)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("dialing %s...", lai.ID)
	err = h.Connect(ctx, *lai)
	if err != nil {
		log.Fatal(err)
	}

	ps := ping.NewPingService(h)
	res := <-ps.Ping(ctx, lai.ID)
	log.Printf("ping: RTT=%s (error=%v)", res.RTT, res.Error)
}

func getRelayedListenerAddr(ctx context.Context) (*peer.AddrInfo, error) {
	rdb := createRdbClient()

	var las string
	for {
		if v, _ := rdb.Get(ctx, "listener/relayedAddr").Result(); v != "" {
			las = v
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	lma, err := ma.NewMultiaddr(las)
	if err != nil {
		log.Fatal(err)
	}
	return peer.AddrInfoFromP2pAddr(lma)
}

func createRdbClient() *redis.Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "registry:6379"
	}
	return redis.NewClient(&redis.Options{Addr: addr})
}
