package connection_manager

import (
	"context"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

type notifiee struct {
	basicNotifee network.Notifiee

	config *Config // +checklocksignore: constant

	peerReputations  map[peer.ID]*peerInfo // +checklocks:mu
	lastUpdateSecond int64                 // +checklocks:mu
	mu               sync.Mutex

	logger zerolog.Logger // +checklocksignore: thread safe
}

var (
	_ network.Notifiee      = (*notifiee)(nil)
	_ PeerReputationTracker = (*notifiee)(nil)
)

func newNotifiee(
	basicNotifee network.Notifiee,
	config *Config,
	logger zerolog.Logger,
) *notifiee {
	if config == nil {
		config = NewDefaultConfig()
	}
	return &notifiee{
		basicNotifee:     basicNotifee,
		config:           config,
		peerReputations:  make(map[peer.ID]*peerInfo),
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

	n.mu.Lock()
	defer n.mu.Unlock()

	n.recalculateReputationsAccordingToCurrentTime()

	closeFunc := func() {
		if err := network.ClosePeer(peer); err != nil {
			n.logger.Error().Err(err).Msgf("Failed to close peer %s", peer)
		}
	}
	var pi *peerInfo
	var ok bool
	if pi, ok = n.peerReputations[peer]; !ok {
		pi = newPeerInfo(peer, 0, closeFunc)
		n.peerReputations[peer] = pi
	} else if pi.closeFunc == nil {
		pi.closeFunc = closeFunc
	}

	if n.isBanned(pi) {
		pi.closePeer(n.logger)
	}
}

func (n *notifiee) Disconnected(network network.Network, connection network.Conn) {
	n.basicNotifee.Disconnected(network, connection)
}

func (n *notifiee) ReportPeer(peer peer.ID, reputationChangeReason reputationChangeReason) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.recalculateReputationsAccordingToCurrentTime()

	pi, ok := n.peerReputations[peer]
	if !ok {
		pi = newPeerInfo(peer, 0, nil)
		n.peerReputations[peer] = pi
	}

	if reputationChangeValue := n.getReputationChange(reputationChangeReason); reputationChangeValue != 0 {
		n.logger.Debug().
			Stringer("peerId", peer).
			Int32("diff", int32(reputationChangeValue)).
			Str("reason", string(reputationChangeReason)).
			Msg("Changing peer reputation")
		pi.reputation = pi.reputation.add(reputationChangeValue)

		if n.isBanned(pi) {
			pi.closePeer(n.logger)
		}
	}
}

func (n *notifiee) isBanned(pi *peerInfo) bool {
	return pi.reputation < n.config.ReputationBanThreshold
}

func (n *notifiee) getReputationChange(reason reputationChangeReason) Reputation {
	if value, ok := n.config.ReputationChangeSettings[reason]; ok {
		return value
	} else {
		n.logger.Error().Str("reason", string(reason)).Msg("Unknown reputation change reason")
	}
	return 0
}

// +checklocks:n.mu
func (n *notifiee) recalculateReputationsAccordingToCurrentTime() {
	currentSecond := n.clock().Now().Unix()
	elapsedSeconds := currentSecond - n.lastUpdateSecond
	n.lastUpdateSecond = currentSecond

	for range elapsedSeconds {
		for _, info := range n.peerReputations {
			info.reputation = n.reputationTick(info.reputation)
		}
	}
	// TODO: We should remove NOT CONNECTED peers with reputation 0.
}

// Exponential decay of reputation
func (n *notifiee) reputationTick(reputation Reputation) Reputation {
	if n.config.DecayReputationPerSecondPercent == 0 {
		return reputation
	}
	diff := Reputation(int(reputation) / int(100/n.config.DecayReputationPerSecondPercent))
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
		ticker := n.clock().NewTicker(time.Second)
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
