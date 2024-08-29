package synccommittee

import (
	"time"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
)

type Config struct {
	RpcEndpoint      string
	OwnRpcEndpoint   string
	PollingDelay     time.Duration
	GracefulShutdown bool
	ProversCount     uint16

	Telemetry *telemetry.Config
}
