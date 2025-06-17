package debug

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
)

// TODO
type HeartbeatSenderMetrics interface {
	Heartbeat(ctx context.Context)
}

type HeartbeatSender struct {
	metrics HeartbeatSenderMetrics
	config  *HeartbeatConfig
	clock   clockwork.Clock
	logger  logging.Logger
}

type HeartbeatConfig struct {
	Interval time.Duration
}

func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Interval: 1 * time.Second,
	}
}

func NewHeartbeatSender(
	config *HeartbeatConfig,
	metrics HeartbeatSenderMetrics,
	clock clockwork.Clock,
	logger logging.Logger,
) *HeartbeatSender {
	hs := &HeartbeatSender{
		metrics: metrics,
		config:  config,
		clock:   clock,
	}

	hs.logger = logger.With().Str(logging.FieldComponent, hs.Name()).Logger()
	return hs
}

func (h *HeartbeatSender) Name() string {
	return "heartbeat-sender"
}

func (h *HeartbeatSender) Run(ctx context.Context, started chan<- struct{}) error {
	eg, gCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		ticker := h.clock.NewTicker(h.config.Interval)
		defer ticker.Stop()
		close(started)

		for {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			case <-ticker.Chan():
				h.metrics.Heartbeat(gCtx)
			}
		}
	})

	return eg.Wait()
}
