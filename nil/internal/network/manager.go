package network

import (
	"context"
)

type Manager interface {
	PubSub() *PubSub
	ProtocolVersion() string
	GetPeerProtocolVersion(peer PeerID) (string, error)
	AllKnownPeers() []PeerID
	GetPeersForProtocol(pid ProtocolID) []PeerID
	Connect(ctx context.Context, addr AddrInfo) (PeerID, error)
	Close()

	NewStream(ctx context.Context, peerId PeerID, protocolId ProtocolID) (Stream, error)
	SetStreamHandler(ctx context.Context, protocolId ProtocolID, handler StreamHandler)
	SetRequestHandler(ctx context.Context, protocolId ProtocolID, handler RequestHandler)
	SendRequestAndGetResponse(ctx context.Context, peerId PeerID, protocolId ProtocolID, request []byte) ([]byte, error)

	getHost() Host
}
