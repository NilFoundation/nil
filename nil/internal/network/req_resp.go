package network

import (
	"context"
	"io"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const (
	streamOpenTimeout = 2 * time.Second
	requestTimeout    = 10 * time.Second
	responseTimeout   = 5 * time.Second
)

type (
	RequestHandler func(context.Context, []byte) ([]byte, error)

	Stream        = network.Stream
	StreamHandler = network.StreamHandler
	ProtocolID    = protocol.ID
)

func (m *Manager) NewStream(ctx context.Context, peerId PeerID, protocolId ProtocolID) (Stream, error) {
	ctx, cancel := context.WithTimeout(ctx, streamOpenTimeout)
	defer cancel()

	return m.host.NewStream(ctx, peerId, protocolId)
}

func (m *Manager) SetStreamHandler(protocolId ProtocolID, handler StreamHandler) {
	m.logger.Debug().Msgf("Setting stream handler for protocol %s", protocolId)

	m.host.SetStreamHandler(protocolId, handler)
}

func (m *Manager) SendRequestAndGetResponse(ctx context.Context, peerId PeerID, protocolId ProtocolID, request []byte) ([]byte, error) {
	stream, err := m.NewStream(ctx, peerId, protocolId)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	if err := stream.SetDeadline(time.Now().Add(requestTimeout)); err != nil {
		return nil, err
	}

	if _, err = stream.Write(request); err != nil {
		return nil, err
	}
	if err := stream.CloseWrite(); err != nil {
		return nil, err
	}

	return io.ReadAll(stream)
}

func (m *Manager) SetRequestHandler(ctx context.Context, protocolId ProtocolID, handler RequestHandler) {
	logger := m.logger.With().Str(logging.FieldProtocolID, string(protocolId)).Logger()

	logger.Debug().Msg("Setting request handler...")

	m.host.SetStreamHandler(protocolId, func(stream network.Stream) {
		defer stream.Close()

		ctx, cancel := context.WithTimeout(ctx, responseTimeout)
		defer cancel()

		m.logger.Trace().Msgf("Handling request %s...", stream.ID())

		if err := stream.SetDeadline(time.Now().Add(responseTimeout)); err != nil {
			m.logError(err, "failed to set deadline for stream")
			return
		}

		request, err := io.ReadAll(stream)
		if err != nil {
			m.logError(err, "failed to read request")
			return
		}

		response, err := handler(ctx, request)
		if err != nil {
			m.logError(err, "failed to handle request")
			return
		}

		if _, err := stream.Write(response); err != nil {
			m.logError(err, "failed to write response")
			return
		}

		m.logger.Trace().Msgf("Handled request %s", stream.ID())
	})
}
