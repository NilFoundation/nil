package network

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

type DHT = dht.IpfsDHT

const (
	startDHTTimeout             = time.Second * 10
	initialAdvertisementTimeout = time.Millisecond
	tryAdvertiseTimeout         = time.Second * 30
	connectToPeersTimeout       = time.Minute
	findPeersTimeout            = time.Second * 5
	defaultMaxPeers             = 50
	discoveryPid                = "/kad"
)

func NewDHT(ctx context.Context, h host.Host, conf *Config, database db.DB, logger logging.Logger) (*DHT, error) {
	if !conf.DHTEnabled {
		return nil, nil
	}

	logger.Debug().Msg("Starting DHT")

	if len(conf.DHTBootstrapPeers) == 0 && conf.DHTMode == dht.ModeClient {
		logger.Warn().Msg("No bootstrap peers provided for DHT in client mode")
	}

	protocol := ProtocolID(conf.Prefix + discoveryPid)

	dhtOpts := []dht.Option{
		dht.Mode(conf.DHTMode),
		dht.BootstrapPeers(ToLibP2pAddrInfoSlice(conf.DHTBootstrapPeers)...),
		dht.RoutingTableRefreshPeriod(1 * time.Minute),
		dht.V1ProtocolOverride(protocol),
	}

	if datastore := db.NewDatastoreFromDB(database, db.DHTTable, nil); datastore != nil {
		dhtOpts = append(dhtOpts, dht.Datastore(datastore))
	}

	res, err := dht.New(
		ctx,
		h,
		dhtOpts...,
	)
	if err != nil {
		return nil, err
	}

	if err := discoverAndAdvertise(ctx, res, h, protocol, logger); err != nil {
		return nil, err
	}

	logger.Info().Msgf("DHT bootstrapped with %d peers", len(conf.DHTBootstrapPeers))

	return res, nil
}

// Almost all discovery/advertisement logic is taken from Polkadot:
// https://github.com/ChainSafe/gossamer/blob/ff33dc50f902b71bb7940a66269ac2bf194a59c7/dot/network/discovery.go
func discoverAndAdvertise(
	ctx context.Context,
	dht *DHT,
	h host.Host,
	protocol ProtocolID,
	logger logging.Logger,
) error {
	rd := routing.NewRoutingDiscovery(dht)

	err := dht.Bootstrap(ctx)
	if err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	// wait to connect to bootstrap peers
	time.Sleep(time.Second)
	go advertise(ctx, rd, dht, protocol, logger)
	go checkPeerCount(ctx, rd, h, protocol, logger)

	logger.Debug().Msg("DHT discovery started")
	return nil
}

func advertise(
	ctx context.Context,
	rd *routing.RoutingDiscovery,
	dht *DHT,
	protocol ProtocolID,
	logger logging.Logger,
) {
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

			ttl, err = rd.Advertise(ctx, string(protocol))
			if err != nil {
				logger.Warn().Err(err).Msg("failed to advertise in the DHT")
				ttl = tryAdvertiseTimeout
			}
		}
	}
}

func checkPeerCount(
	ctx context.Context,
	rd *routing.RoutingDiscovery,
	h host.Host,
	protocol ProtocolID,
	logger logging.Logger,
) {
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

			findPeers(ctx, rd, h, protocol, logger)
		}
	}
}

func findPeers(
	ctx context.Context,
	rd *routing.RoutingDiscovery,
	h host.Host,
	protocol ProtocolID,
	logger logging.Logger,
) {
	logger.Debug().Msg("attempting to find DHT peers...")

	ctx, cancel := context.WithTimeout(ctx, findPeersTimeout)
	defer cancel()
	peerCh, err := rd.FindPeers(ctx, string(protocol))
	if err != nil {
		logger.Warn().Err(err).Msg("failed to begin finding peers via DHT")
		return
	}

	for {
		select {
		case <-ctx.Done():
			logger.Debug().Msg("findPeers: timer expired")
			return
		case peer, ok := <-peerCh:
			if !ok {
				logger.Trace().Msg("findPeers: peer channel closed")
				return
			}

			logger.Trace().Msgf("findPeers: received peer %s", peer.ID)
			if peer.ID == h.ID() || peer.ID == "" {
				continue
			}

			logger.Debug().Msgf("found new peer %s via DHT", peer.ID)
			h.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
			// TODO: add [peerset](
			//  https://github.com/ChainSafe/gossamer/tree/ff33dc50f902b71bb7940a66269ac2bf194a59c7/dot/peerset)
		}
	}
}
