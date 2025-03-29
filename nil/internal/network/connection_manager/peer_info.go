package connection_manager

import (
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
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
	reputation     Reputation
	disconnectedAt *time.Time
	logger         logging.Logger
	closeFunc      func()
}

func (pi *peerInfo) closePeer() {
	if pi.closeFunc != nil {
		pi.closeFunc()
	} else {
		pi.logger.Warn().Msg("Trying to close peer which wasn't ever connected")
	}
}

func newPeerInfo(reputation Reputation, logger logging.Logger, closePeer func()) *peerInfo {
	return &peerInfo{
		reputation: reputation,
		logger:     logger,
		closeFunc:  closePeer,
	}
}
