package connection_manager

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
)

// With a normal peerInfo life cycle, it is created when connecting to the peer.
// At this moment, we have access to network, respectively, we can remember the close function
// that requires access to it.
// This is important because we will want to disconnect from the peer at the time
// of a decrease in the reputation below the threshold,
// and in this context we no longer have access to the network.
// Nevertheless, if for some reason the command to reduce the reputation for the peer
// will come to its connection, then we will not have closeFunc.
// This is not scary, since we are not yet connected to the peer and do not need to do anything.
// If later an attempt to connect will occur, then we will install closeFunc
// and will be able to use it if necessary.
type peerInfo struct {
	id         peer.ID
	reputation Reputation
	closeFunc  func()
}

func (pi *peerInfo) closePeer(logger zerolog.Logger) {
	if pi.closeFunc != nil {
		logger.Debug().Stringer("peerId", pi.id).Msg("Disconnecting banned peer")
		pi.closeFunc()
	} else {
		logger.Warn().Msg("Trying to close peer which wasn't ever connected")
	}
}

func newPeerInfo(peerId peer.ID, reputation Reputation, closePeer func()) *peerInfo {
	return &peerInfo{
		id:         peerId,
		reputation: reputation,
		closeFunc:  closePeer,
	}
}
