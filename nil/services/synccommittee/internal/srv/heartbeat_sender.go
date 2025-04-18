package srv

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
)

type HeartbeatSenderMetrics interface {
	WorkerMetrics
	Heartbeat(ctx context.Context)
}

type HeartbeatSenderConfig struct {
	Interval time.Duration
}

func DefaultHeartbeatSenderConfig() HeartbeatSenderConfig {
	return HeartbeatSenderConfig{
		Interval: 5 * time.Second,
	}
}

func NewHeartbeatSender(
	metrics HeartbeatSenderMetrics,
	config HeartbeatSenderConfig,
	logger logging.Logger,
) Worker {
	loopConfig := NewWorkerLoopConfig(
		"heartbeat_sender",
		config.Interval,
		func(ctx context.Context) error {
			metrics.Heartbeat(ctx)
			return nil
		})

	workerLoop := NewWorkerLoop(loopConfig, metrics, logger)
	return &workerLoop
}
