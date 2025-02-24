package network

import (
	"context"
	"errors"
	"io"
	"runtime/debug"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
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

type stream struct {
	network.Stream

	measurer *telemetry.Measurer
}

func (s *stream) Close() error {
	s.measurer.Measure(context.Background())
	return s.Stream.Close()
}

func (m *Manager) NewStream(ctx context.Context, peerId PeerID, protocolId ProtocolID) (Stream, error) {
	ctx, cancel := context.WithTimeout(ctx, streamOpenTimeout)
	defer cancel()

	protocolId = ProtocolID(m.withNetworkPrefix(string(protocolId)))
	s, err := m.host.NewStream(ctx, peerId, protocolId)
	if err != nil {
		return nil, err
	}

	measurer, err := telemetry.NewMeasurer(m.meter, "out_streams",
		telattr.P2PIdentity(m.host.ID()),
		telattr.ProtocolId(protocolId),
		telattr.PeerId(peerId))
	if err != nil {
		return nil, err
	}

	return &stream{s, measurer}, nil
}

func (m *Manager) SetStreamHandler(ctx context.Context, protocolId ProtocolID, handler StreamHandler) {
	m.logger.Debug().Msgf("Setting stream handler for protocol %s", protocolId)

	protocolId = ProtocolID(m.withNetworkPrefix(string(protocolId)))
	m.host.SetStreamHandler(protocolId, func(stream Stream) {
		defer stream.Close()

		measurer, err := telemetry.NewMeasurer(m.meter, "in_streams",
			telattr.P2PIdentity(m.host.ID()),
			telattr.ProtocolId(protocolId),
			telattr.PeerId(stream.Conn().RemotePeer()))
		if err != nil {
			m.logError(err, "Failed to create measurer for incoming stream")
		} else {
			defer measurer.Measure(ctx)
		}

		handler(stream)
	})
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

	m.SetStreamHandler(ctx, protocolId, func(stream Stream) {
		ctx, cancel := context.WithTimeout(ctx, responseTimeout)
		defer cancel()

		logger.Trace().Msgf("Handling request %s...", stream.ID())

		if err := stream.SetDeadline(time.Now().Add(responseTimeout)); err != nil {
			m.logErrorWithLogger(logger, err, "Failed to set deadline for stream")
			return
		}

		request, err := io.ReadAll(stream)
		if err != nil {
			m.logErrorWithLogger(logger, err, "Failed to read request")
			return
		}

		var response []byte
		err = func() (errRes error) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error().Msgf("Request handler crashed: %v. Stack:\n%s", err, string(debug.Stack()))
					errRes = errors.New("method handler crashed")
				}
			}()

			response, err = handler(ctx, request)
			return err
		}()
		if err != nil {
			m.logErrorWithLogger(logger, err, "Failed to handle request")
			return
		}

		if _, err := stream.Write(response); err != nil {
			m.logErrorWithLogger(logger, err, "Failed to write response")
			return
		}

		logger.Trace().Msgf("Handled request %s", stream.ID())
	})
}
