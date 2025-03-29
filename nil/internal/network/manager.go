package network

import (
	"context"
	"slices"
	"strings"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
)

type Manager struct {
	ctx    context.Context
	prefix string

	host   Host
	pubSub *PubSub
	dht    *DHT

	meter telemetry.Meter

	logger logging.Logger
}

func ConnectToPeers(ctx context.Context, peers AddrInfoSlice, m Manager, logger logging.Logger) {
	connectToPeers(ctx, peers, m.host, logger)
}

func connectToPeers(ctx context.Context, peers AddrInfoSlice, h Host, logger logging.Logger) {
	for _, peerInfo := range peers {
		if h.ID() == peerInfo.ID {
			// Skip connecting to self.
			continue
		}

		if err := h.Connect(ctx, peer.AddrInfo(peerInfo)); err != nil {
			logger.Warn().Err(err).Msgf("Failed to connect to %s", peerInfo)
		}

		h.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.AddressTTL)
	}
}

func connectToDhtBootstrapPeers(ctx context.Context, conf *Config, h Host, logger logging.Logger) {
	connectToPeers(ctx, conf.DHTBootstrapPeers, h, logger)
}

func newManagerFromHost(
	ctx context.Context,
	conf *Config,
	h host.Host,
	database db.DB,
	logger logging.Logger,
) (*Manager, error) {
	logger.Info().Msgf("Listening on addresses:\n%s\n", common.Join("\n", h.Addrs()...))

	connectToDhtBootstrapPeers(ctx, conf, h, logger)

	dht, err := NewDHT(ctx, h, conf, database, logger)
	if err != nil {
		return nil, err
	}

	ps, err := newPubSub(ctx, h, conf, logger)
	if err != nil {
		return nil, err
	}

	return &Manager{
		ctx:    ctx,
		prefix: conf.Prefix,
		host:   h,
		pubSub: ps,
		dht:    dht,
		meter:  telemetry.NewMeter("github.com/NilFoundation/nil/nil/internal/network"),
		logger: logger,
	}, nil
}

func (m *Manager) withNetworkPrefix(prefix string) string {
	return m.prefix + prefix
}

func NewManager(ctx context.Context, conf *Config, database db.DB) (*Manager, error) {
	if !conf.Enabled() {
		return nil, ErrNetworkDisabled
	}

	if conf.PrivateKey == nil {
		if conf.KeysPath == "" {
			return nil, ErrPrivateKeyMissing
		}

		privateKey, err := LoadOrGenerateKeys(conf.KeysPath)
		if err != nil {
			return nil, err
		}
		conf.PrivateKey = privateKey
	}

	h, logger, err := newHost(ctx, conf)
	if err != nil {
		return nil, err
	}
	return newManagerFromHost(ctx, conf, h, database, logger)
}

func NewClientManager(ctx context.Context, conf *Config, database db.DB) (*Manager, error) {
	h, logger, err := newClient(ctx, conf)
	if err != nil {
		return nil, err
	}
	return newManagerFromHost(ctx, conf, h, database, logger)
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

	pid = ProtocolID(m.withNetworkPrefix(string(pid)))
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

	prefix = m.withNetworkPrefix(prefix)
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

func (m *Manager) Connect(ctx context.Context, addr AddrInfo) (PeerID, error) {
	m.logger.Debug().Msgf("Connecting to %s", addr)

	if err := m.host.Connect(ctx, peer.AddrInfo(addr)); err != nil {
		return "", err
	}
	return addr.ID, nil
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

func (m *Manager) logErrorWithLogger(logger logging.Logger, err error, msg string) {
	if m.ctx.Err() != nil {
		// If we're already closing, no need to log errors.
		return
	}

	logger.Error().Err(err).Msg(msg)
}
