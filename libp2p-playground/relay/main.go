package main

import (
	"context"
	"errors"
	"github.com/libp2p/go-libp2p/core/host"
	"log"
	"net"
	"os"

	libp2plog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/redis/go-redis/v9"
)

func main() {
	libp2plog.SetDebugLogging()

	ctx := context.Background()

	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/4003"),
		// Provide a service to peers for determining their reachability status.
		// This is necessary so that the connecting peer knows that its reachability is private
		// and starts the relay finder.
		libp2p.EnableNATService(),
		// RelayManager will create Relay only if our reachability is public.
		libp2p.ForceReachabilityPublic(),
		libp2p.EnableRelayService(relay.WithInfiniteLimits()),
	)
	if err != nil {
		log.Fatalf("host init: %v", err)
	}
	defer h.Close()

	addr, err := getAddress(h)
	if err != nil {
		log.Fatal(err)
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "registry:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := rdb.Set(ctx, "relay/addr", addr.String(), 0).Err(); err != nil {
		log.Fatalf("redis SET: %v", err)
	}
	log.Printf("relay ready - id=%s addrs=%v", h.ID(), h.Addrs())

	<-ctx.Done()
}

func getAddress(h host.Host) (ma.Multiaddr, error) {
	var transport ma.Multiaddr
	for _, a := range h.Addrs() {
		if isLoopback(a) {
			continue
		}
		transport = a
		break
	}
	if transport == nil {
		return nil, errors.New("no non-loopback addr found to publish")
	}
	return transport.Encapsulate(ma.StringCast("/p2p/" + h.ID().String())), nil
}

func isLoopback(m ma.Multiaddr) bool {
	v4, err := m.ValueForProtocol(ma.P_IP4)
	if err == nil {
		return net.ParseIP(v4).IsLoopback()
	}
	v6, err := m.ValueForProtocol(ma.P_IP6)
	if err == nil {
		return net.ParseIP(v6).IsLoopback()
	}
	return false
}
