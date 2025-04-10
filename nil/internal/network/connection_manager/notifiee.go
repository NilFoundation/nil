package connection_manager

import (
	"context"
	"sync"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/jonboulle/clockwork"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type notifiee struct {
	basicNotifee network.Notifiee

	config *Config // +checklocksignore: constant

	peerInfos        map[peer.ID]*peerInfo // +checklocks:mu
	lastUpdateSecond int64                 // +checklocks:mu
	mu               sync.Mutex

	logger logging.Logger // +checklocksignore: thread safe
}

var (
	_ network.Notifiee      = (*notifiee)(nil)
	_ PeerReputationTracker = (*notifiee)(nil)
)

func newNotifiee(
	basicNotifee network.Notifiee,
	config *Config,
	logger logging.Logger,
) *notifiee {
	if config == nil {
		config = NewDefaultConfig()
	}
	return &notifiee{
		basicNotifee:     basicNotifee,
		config:           config,
		peerInfos:        make(map[peer.ID]*peerInfo),
		lastUpdateSecond: config.clock.Now().Unix(),
		logger:           logger,
	}
}

func (n *notifiee) Listen(network network.Network, address ma.Multiaddr) {
	n.basicNotifee.Listen(network, address)
}

func (n *notifiee) ListenClose(network network.Network, address ma.Multiaddr) {
	n.basicNotifee.ListenClose(network, address)
}

func (n *notifiee) Connected(network network.Network, connection network.Conn) {
	n.basicNotifee.Connected(network, connection)

	peer := connection.RemotePeer()
	peerLogger := n.logger.With().Stringer(logging.FieldPeerId, peer).Logger()

	n.mu.Lock()
	defer n.mu.Unlock()

	n.recalculateReputationsAccordingToCurrentTime()

	var pi *peerInfo
	var ok bool
	if pi, ok = n.peerInfos[peer]; !ok {
		pi = newPeerInfo(0, peerLogger, nil)
		n.peerInfos[peer] = pi
	}
	if pi.closeFunc == nil {
		pi.closeFunc = func() {
			peerLogger.Debug().Msg("Disconnecting banned peer")
			if err := network.ClosePeer(peer); err != nil {
				peerLogger.Error().Err(err).Msg("Failed to close peer")
			}
		}
	}
	pi.disconnectedAt = nil

	if n.isBanned(pi) {
		pi.closePeer()
	}
}

func (n *notifiee) Disconnected(network network.Network, connection network.Conn) {
	n.basicNotifee.Disconnected(network, connection)

	n.mu.Lock()
	defer n.mu.Unlock()

	peer := connection.RemotePeer()
	pi, ok := n.peerInfos[peer]
	if !ok {
		n.logger.Warn().Stringer(logging.FieldPeerId, peer).Msg("Disconnected from unknown peer")
		return
	}
	now := n.clock().Now()
	pi.disconnectedAt = &now
}

func (n *notifiee) ReportPeer(peer peer.ID, reputationChangeReason reputationChangeReason) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.recalculateReputationsAccordingToCurrentTime()

	pi, ok := n.peerInfos[peer]
	if !ok {
		pi = newPeerInfo(0, n.logger, nil)
		n.peerInfos[peer] = pi
	}

	if reputationChangeValue := n.getReputationChange(reputationChangeReason); reputationChangeValue != 0 {
		n.logger.Debug().
			Stringer(logging.FieldPeerId, peer).
			Int32("diff", int32(reputationChangeValue)).
			Str("reason", string(reputationChangeReason)).
			Msg("Changing peer reputation")
		pi.reputation = pi.reputation.add(reputationChangeValue)

		if n.isBanned(pi) {
			pi.closePeer()
		}
	}
}

func (n *notifiee) isBanned(pi *peerInfo) bool {
	return pi.reputation < n.config.ReputationBanThreshold
}

func (n *notifiee) getReputationChange(reason reputationChangeReason) Reputation {
	if value, ok := n.config.ReputationChangeSettings[reason]; ok {
		return value
	}
	n.logger.Error().Str("reason", string(reason)).Msg("Unknown reputation change reason")
	return 0
}

// +checklocks:n.mu
func (n *notifiee) recalculateReputationsAccordingToCurrentTime() {
	currentSecond := n.clock().Now().Unix()
	elapsedSeconds := currentSecond - n.lastUpdateSecond
	n.lastUpdateSecond = currentSecond

	for range elapsedSeconds {
		for _, info := range n.peerInfos {
			info.reputation = n.reputationTick(info.reputation)
		}
	}
	for peer, info := range n.peerInfos {
		if info.disconnectedAt != nil && (info.reputation == 0 ||
			info.disconnectedAt.Add(n.config.ForgetAfterTime).Before(n.clock().Now())) {
			delete(n.peerInfos, peer)
		}
	}
}

// Exponential decay of reputation
func (n *notifiee) reputationTick(reputation Reputation) Reputation {
	if n.config.DecayReputationPerSecondPercent == 0 {
		return reputation
	}
	diff := Reputation(int(reputation) * int(n.config.DecayReputationPerSecondPercent) / 100)
	if diff == 0 && reputation < 0 {
		diff = -1
	} else if diff == 0 && reputation > 0 {
		diff = 1
	}
	return reputation.sub(diff)
}

func (n *notifiee) clock() clockwork.Clock {
	return n.config.clock
}

func (n *notifiee) start(ctx context.Context) {
	go func() {
		ticker := n.clock().NewTicker(n.config.RecalculateReputationsTimeout)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.Chan():
				func() {
					n.mu.Lock()
					defer n.mu.Unlock()

					n.recalculateReputationsAccordingToCurrentTime()
				}()
			}
		}
	}()
}
