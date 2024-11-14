package internal

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var _ metrics.Reporter = (*MetricsReporter)(nil)

type MetricsReporter struct {
	ctx context.Context
	id  peer.ID

	sentSize telemetry.Counter
	recvSize telemetry.Counter
}

func NewMetricsReporter(ctx context.Context, id peer.ID) (*MetricsReporter, error) {
	meter := telemetry.NewMeter("github.com/NilFoundation/nil/nil/internal/network")

	sentSize, err := meter.Int64Counter("sent_size")
	if err != nil {
		return nil, err
	}
	recvSize, err := meter.Int64Counter("recv_size")
	if err != nil {
		return nil, err
	}
	return &MetricsReporter{
		ctx:      ctx,
		id:       id,
		sentSize: sentSize,
		recvSize: recvSize,
	}, nil
}

func (s *MetricsReporter) LogSentMessage(int64) {
	// handled by LogSentMessageStream
}

func (s *MetricsReporter) LogRecvMessage(int64) {
	// handled by LogRecvMessageStream
}

func (s *MetricsReporter) LogSentMessageStream(size int64, protocol protocol.ID, peer peer.ID) {
	s.sentSize.Add(s.ctx, size, telattr.With(
		telattr.P2PIdentity(s.id),
		telattr.PeerId(peer),
		telattr.ProtocolId(protocol),
	))
}

func (s *MetricsReporter) LogRecvMessageStream(size int64, protocol protocol.ID, peer peer.ID) {
	s.recvSize.Add(s.ctx, size, telattr.With(
		telattr.P2PIdentity(s.id),
		telattr.PeerId(peer),
		telattr.ProtocolId(protocol),
	))
}

func (s *MetricsReporter) GetBandwidthForPeer(peer.ID) metrics.Stats {
	panic("not implemented")
}

func (s *MetricsReporter) GetBandwidthForProtocol(protocol.ID) metrics.Stats {
	panic("not implemented")
}

func (s *MetricsReporter) GetBandwidthTotals() metrics.Stats {
	panic("not implemented")
}

func (s *MetricsReporter) GetBandwidthByPeer() map[peer.ID]metrics.Stats {
	panic("not implemented")
}

func (s *MetricsReporter) GetBandwidthByProtocol() map[protocol.ID]metrics.Stats {
	panic("not implemented")
}
