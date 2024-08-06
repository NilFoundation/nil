package network

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/rs/zerolog"
)

const MdnsServiceTag = "nil-nodes"

// setupMdnsDiscovery sets up mDNS discovery for the given host.
// The service must be closed after use.
// It works in LANs and is useful for local development.
func setupMdnsDiscovery(ctx context.Context, h Host) (mdns.Service, error) {
	s := mdns.NewMdnsService(h, MdnsServiceTag, &discoveryNotifee{
		ctx:    ctx,
		h:      h,
		logger: logging.NewLogger("mdns"),
	})
	if err := s.Start(); err != nil {
		return nil, err
	}
	return s, nil
}

// discoveryNotifee gets notified when we find a new peer via mDNS discovery
type discoveryNotifee struct {
	ctx    context.Context
	h      host.Host
	logger zerolog.Logger
}

// HandlePeerFound connects to peers discovered via mDNS.
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	n.logger.Debug().
		Stringer(logging.FieldPeerId, pi.ID).
		Msg("discovered new peer")
	if err := n.h.Connect(n.ctx, pi); err != nil {
		n.logger.Error().
			Err(err).
			Stringer(logging.FieldPeerId, pi.ID).
			Msg("error connecting to peer")
	}
}
