package network

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	cm "github.com/NilFoundation/nil/nil/internal/network/connection_manager"
	"github.com/NilFoundation/nil/nil/internal/network/internal"
	"github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
)

type Host = host.Host

var defaultGracePeriod = connmgr.WithGracePeriod(time.Minute)

func getCommonOptions(ctx context.Context, conf *Config) ([]libp2p.Option, logging.Logger, error) {
	pid, err := peer.IDFromPublicKey(conf.PrivateKey.GetPublic())
	if err != nil {
		return nil, logging.Nop(), err
	}

	logger := internal.Logger.With().
		Stringer(logging.FieldP2PIdentity, pid).
		Logger()

	cm, err := cm.NewConnectionManagerWithPeerReputationTracking(
		ctx,
		conf.ConnectionManagerConfig,
		logger,
		// lo and hi are watermarks governing the number of connections that'll be maintained.
		// When the peer count exceeds the 'high watermark', as many peers will be pruned (and
		// their connections terminated) until 'low watermark' peers remain.
		100, // low
		400, // hi
		defaultGracePeriod)
	if err != nil {
		return nil, logger, err
	}

	metrics, err := internal.NewMetricsReporter(ctx, pid)
	if err != nil {
		return nil, logger, err
	}

	return []libp2p.Option{
		libp2p.Security(noise.ID, noise.New),
		libp2p.ConnectionManager(cm),
		libp2p.Identity(conf.PrivateKey),
		libp2p.BandwidthReporter(metrics),
	}, logger, nil
}

// newHost creates a new libp2p host. It must be closed after use.
func newHost(ctx context.Context, conf *Config) (Host, logging.Logger, error) {
	addr := conf.IPV4Address
	if addr == "" {
		addr = "0.0.0.0"
	}

	options, logger, err := getCommonOptions(ctx, conf)
	if err != nil {
		return nil, logging.Nop(), err
	}

	if conf.TcpPort != 0 {
		options = append(options,
			libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", addr, conf.TcpPort)),
			libp2p.Transport(tcp.NewTCPTransport),
		)
	}

	if conf.QuicPort != 0 {
		options = append(options,
			libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/udp/%d/quic", addr, conf.QuicPort)),
			libp2p.Transport(quic.NewTransport),
		)
	}

	if conf.ServeRelay {
		options = append(options, libp2p.EnableRelayService())

		// todo: remove it after relay is tested
		// this is to make sure that the relay is not disabled
		if conf.Reachability == network.ReachabilityUnknown {
			options = append(options, libp2p.ForceReachabilityPublic())
		}
	}

	if len(conf.Relays) > 0 {
		options = append(options, libp2p.EnableAutoRelayWithStaticRelays(ToLibP2pAddrInfoSlice(conf.Relays)))
	}

	// In tests, we might wish to force a specific reachability
	switch conf.Reachability {
	case network.ReachabilityUnknown:
		// default
	case network.ReachabilityPublic:
		options = append(options, libp2p.ForceReachabilityPublic())
	case network.ReachabilityPrivate:
		options = append(options, libp2p.ForceReachabilityPrivate())
	}

	host, err := libp2p.New(options...)
	if err != nil {
		return nil, logging.Nop(), err
	}
	return host, logger, nil
}

// newClient creates a new libp2p host that doesn't listen to any port. It must be closed after use.
func newClient(ctx context.Context, conf *Config) (Host, logging.Logger, error) {
	var privateKey libp2pcrypto.PrivKey
	if conf != nil && conf.PrivateKey != nil {
		privateKey = conf.PrivateKey
	}
	if privateKey == nil {
		var err error
		privateKey, err = GeneratePrivateKey()
		if err != nil {
			return nil, logging.Nop(), err
		}
	}

	options, logger, err := getCommonOptions(ctx, &Config{PrivateKey: privateKey})
	if err != nil {
		return nil, logging.Nop(), err
	}
	options = append(options, libp2p.NoListenAddrs)
	host, err := libp2p.New(options...)
	if err != nil {
		return nil, logging.Nop(), err
	}
	return host, logger, nil
}
