package network

import (
	"context"
	"fmt"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/rs/zerolog"
)

type DHT = dht.IpfsDHT

const (
	startDHTTimeout             = time.Second * 10
	initialAdvertisementTimeout = time.Millisecond
	tryAdvertiseTimeout         = time.Second * 30
	connectToPeersTimeout       = time.Minute
	findPeersTimeout            = time.Second * 5
	defaultMaxPeers             = 50
	discoveryPid                = ProtocolID("/nil/kad")
)

func NewDHT(ctx context.Context, h host.Host, conf *Config, logger zerolog.Logger) (*DHT, error) {
	if !conf.DHTEnabled {
		return nil, nil
	}

	logger.Debug().Msg("Starting DHT")

	if len(conf.DHTBootstrapPeers) == 0 && conf.DHTMode == dht.ModeClient {
		logger.Warn().Msg("No bootstrap peers provided for DHT in client mode")
	}

	res, err := dht.New(
		ctx,
		h,
		dht.Mode(conf.DHTMode),
		dht.BootstrapPeers(ToLibP2pAddrInfoSlice(conf.DHTBootstrapPeers)...),
		dht.RoutingTableRefreshPeriod(1*time.Minute),
		dht.V1ProtocolOverride(discoveryPid),
	)
	if err != nil {
		return nil, err
	}

	if err := discoverAndAdvertise(ctx, res, h, logger); err != nil {
		return nil, err
	}

	logger.Info().Msgf("DHT bootstrapped with %d peers", len(conf.DHTBootstrapPeers))

	return res, nil
}

// Almost all discovery/advertisement logic is taken from Polkadot:
// https://github.com/ChainSafe/gossamer/blob/ff33dc50f902b71bb7940a66269ac2bf194a59c7/dot/network/discovery.go
func discoverAndAdvertise(ctx context.Context, dht *DHT, h host.Host, logger zerolog.Logger) error {
	rd := routing.NewRoutingDiscovery(dht)

	err := dht.Bootstrap(ctx)
	if err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	// wait to connect to bootstrap peers
	time.Sleep(time.Second)
	go advertise(ctx, rd, dht, logger)
	go checkPeerCount(ctx, rd, h, logger)

	logger.Debug().Msg("DHT discovery started")
	return nil
}

func advertise(ctx context.Context, rd *routing.RoutingDiscovery, dht *DHT, logger zerolog.Logger) {
	ttl := initialAdvertisementTimeout

	for {
		timer := time.NewTimer(ttl)

		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			logger.Debug().Msg("advertising ourselves in the DHT...")
			err := dht.Bootstrap(ctx)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to bootstrap DHT")
				continue
			}

			ttl, err = rd.Advertise(ctx, string(discoveryPid))
			if err != nil {
				logger.Warn().Err(err).Msg("failed to advertise in the DHT")
				ttl = tryAdvertiseTimeout
			}
		}
	}
}

func checkPeerCount(ctx context.Context, rd *routing.RoutingDiscovery, h host.Host, logger zerolog.Logger) {
	ticker := time.NewTicker(connectToPeersTimeout)
	maxPeers := defaultMaxPeers
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(h.Network().Peers()) >= maxPeers {
				continue
			}

			findPeers(ctx, rd, h, logger)
		}
	}
}

func findPeers(ctx context.Context, rd *routing.RoutingDiscovery, h host.Host, logger zerolog.Logger) {
	logger.Debug().Msg("attempting to find DHT peers...")

	ctx, cancel := context.WithTimeout(ctx, findPeersTimeout)
	defer cancel()
	peerCh, err := rd.FindPeers(ctx, string(discoveryPid))
	if err != nil {
		logger.Warn().Err(err).Msgf("failed to begin finding peers via DHT")
		return
	}

	for {
		select {
		case <-ctx.Done():
			logger.Debug().Msgf("findPeers: timer expired")
			return
		case peer := <-peerCh:
			logger.Trace().Msgf("findPeers: received peer %s", peer.ID)
			if peer.ID == h.ID() || peer.ID == "" {
				continue
			}

			logger.Debug().Msgf("found new peer %s via DHT", peer.ID)
			h.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
			// TODO: add peerset (https://github.com/ChainSafe/gossamer/tree/ff33dc50f902b71bb7940a66269ac2bf194a59c7/dot/peerset)
		}
	}
}
