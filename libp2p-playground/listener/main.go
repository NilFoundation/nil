package main

import (
	"context"
	"github.com/libp2p/go-libp2p/core/host"
	"log"
	"os"
	"time"

	libp2plog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/redis/go-redis/v9"
)

func main() {
	libp2plog.SetDebugLogging()

	ctx := context.Background()
	rdb := createRdbClient()
	relayAddr, err := getRelayAddress(ctx, rdb)
	if err != nil {
		log.Fatal(err)
	}

	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/4004"),
		libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*relayAddr}),
		// This is necessary so that when connecting to a node that supports the autonat protocol (relay),
		// we find out that our reachability is private and launch the relay finder.
		libp2p.EnableAutoNATv2(),
	)
	if err != nil {
		log.Fatal(err)
	}

	err = h.Connect(ctx, *relayAddr)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("host up; id=%s, waiting for relay address…", h.ID())
	relayedAddr := getRelayedAddress(h)

	_ = rdb.Set(ctx, "listener/relayedAddr", relayedAddr.String(), 0).Err()
	log.Printf("listener ready; relay-addr=%s", relayedAddr)

	<-ctx.Done()
}

func getRelayAddress(ctx context.Context, rdb *redis.Client) (*peer.AddrInfo, error) {
	relayStr, err := rdb.Get(ctx, "relay/addr").Result()
	if err != nil {
		log.Fatal(err)
	}
	relayMA, err := ma.NewMultiaddr(relayStr)
	if err != nil {
		log.Fatal(err)
	}
	return peer.AddrInfoFromP2pAddr(relayMA)
}

func getRelayedAddress(h host.Host) ma.Multiaddr {
	var relayed ma.Multiaddr
	for {
		log.Printf("addrs: %v\n", h.Addrs())
		for _, a := range h.Addrs() {
			if _, err := a.ValueForProtocol(ma.P_CIRCUIT); err == nil {
				relayed = a
				break
			}
		}
		if relayed != nil {
			relayed = relayed.Encapsulate(ma.StringCast("/p2p/" + h.ID().String()))
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	return relayed
}

func createRdbClient() *redis.Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "registry:6379"
	}
	return redis.NewClient(&redis.Options{Addr: addr})
}
