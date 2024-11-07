package network

import (
	"context"
	"slices"
	"strings"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/network/internal"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"
)

type Manager struct {
	ctx context.Context

	host   Host
	pubSub *PubSub
	dht    *DHT

	meter telemetry.Meter

	logger zerolog.Logger
}

func connectToBootstrapPeers(ctx context.Context, conf *Config, h Host, logger zerolog.Logger) error {
	for _, p := range conf.DHTBootstrapPeers {
		peerInfo, err := peer.AddrInfoFromString(p)
		if err != nil {
			return err
		}

		if err := h.Connect(ctx, *peerInfo); err != nil {
			logger.Warn().Err(err).Msgf("Failed to connect to %s", p)
		}

		h.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.AddressTTL)
	}
	return nil
}

func newManagerFromHost(ctx context.Context, conf *Config, h host.Host) (*Manager, error) {
	logger := internal.Logger.With().
		Stringer(logging.FieldP2PIdentity, h.ID()).
		Logger()

	logger.Info().Msgf("Listening on addresses:\n%s\n", common.Join("\n", h.Addrs()...))

	if err := connectToBootstrapPeers(ctx, conf, h, logger); err != nil {
		return nil, err
	}

	dht, err := NewDHT(ctx, h, conf, logger)
	if err != nil {
		return nil, err
	}

	ps, err := newPubSub(ctx, h, logger)
	if err != nil {
		return nil, err
	}

	return &Manager{
		ctx:    ctx,
		host:   h,
		pubSub: ps,
		dht:    dht,
		meter:  telemetry.NewMeter("github.com/NilFoundation/nil/nil/internal/network"),
		logger: logger,
	}, nil
}

func NewManager(ctx context.Context, conf *Config) (*Manager, error) {
	if !conf.Enabled() {
		return nil, ErrNetworkDisabled
	}

	if conf.PrivateKey == nil {
		return nil, ErrPrivateKeyMissing
	}

	h, err := newHost(conf)
	if err != nil {
		return nil, err
	}
	return newManagerFromHost(ctx, conf, h)
}

func NewClientManager(ctx context.Context, conf *Config) (*Manager, error) {
	h, err := newClient(conf)
	if err != nil {
		return nil, err
	}
	return newManagerFromHost(ctx, conf, h)
}

func (m *Manager) PubSub() *PubSub {
	return m.pubSub
}

func (m *Manager) AllKnownPeers() []peer.ID {
	return slices.DeleteFunc(m.host.Peerstore().PeersWithAddrs(), func(i PeerID) bool {
		return m.host.ID() == i
	})
}

func (m *Manager) GetPeersForProtocol(pid protocol.ID) []peer.ID {
	var peersForProtocol []peer.ID
	peers := m.host.Network().Peers()

	for _, p := range peers {
		supportedProtocols, err := m.host.Peerstore().SupportsProtocols(p, pid)
		if err == nil && len(supportedProtocols) > 0 {
			peersForProtocol = append(peersForProtocol, p)
		}
	}

	return peersForProtocol
}

func (m *Manager) GetPeersForProtocolPrefix(prefix string) []peer.ID {
	if len(prefix) == 0 || prefix[len(prefix)-1] != '/' {
		m.logger.Error().Msgf("Invalid protocol prefix: %s. It should be a string ending with '/'", prefix)
		return nil
	}

	var peersForProtocolPrefix []peer.ID
	peers := m.host.Network().Peers()

	for _, p := range peers {
		supportedProtocols, err := m.host.Peerstore().GetProtocols(p)
		if err == nil && len(supportedProtocols) > 0 {
			for _, sp := range supportedProtocols {
				if strings.HasPrefix(string(sp), prefix) {
					peersForProtocolPrefix = append(peersForProtocolPrefix, p)
					break
				}
			}
		}
	}
	return peersForProtocolPrefix
}

func (m *Manager) Connect(ctx context.Context, addr string) (PeerID, error) {
	m.logger.Debug().Msgf("Connecting to %s", addr)

	addrInfo, err := peer.AddrInfoFromString(addr)
	if err != nil {
		return "", err
	}
	if err := m.host.Connect(ctx, *addrInfo); err != nil {
		return "", err
	}
	return addrInfo.ID, nil
}

func (m *Manager) Close() {
	if m.dht != nil {
		if err := m.dht.Close(); err != nil {
			m.logError(err, "Error closing DHT")
		}
	}

	if m.pubSub != nil {
		if err := m.pubSub.Close(); err != nil {
			m.logError(err, "Error closing pubsub")
		}
	}

	if err := m.host.Close(); err != nil {
		m.logError(err, "Error closing host")
	}
}

func (m *Manager) logError(err error, msg string) {
	m.logErrorWithLogger(m.logger, err, msg)
}

func (m *Manager) logErrorWithLogger(logger zerolog.Logger, err error, msg string) {
	if m.ctx.Err() != nil {
		// If we're already closing, no need to log errors.
		return
	}

	logger.Error().Err(err).Msg(msg)
}
