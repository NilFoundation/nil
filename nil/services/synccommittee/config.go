package synccommittee

import (
	"time"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
)

type Config struct {
	RpcEndpoint      string
	PollingDelay     time.Duration
	GracefulShutdown bool

	Telemetry *telemetry.Config
}
