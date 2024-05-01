package httpcfg

import (
	"time"

	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
)

type HttpCfg struct {
	Enabled bool

	AuthRpcHTTPListenAddress string

	HttpURL            string
	HttpListenAddress  string
	HttpPort           int
	HttpCORSDomain     []string
	HttpVirtualHost    []string
	AuthRpcVirtualHost []string
	HttpCompression    bool

	AuthRpcPort int

	API                 []string
	RpcBatchConcurrency uint

	JWTSecretPath      string // Engine API Authentication
	TraceRequests      bool   // Print requests to logs at INFO level
	DebugSingleRequest bool   // Print single-request-related debugging info to logs at INFO level
	HTTPTimeouts       rpccfg.HTTPTimeouts
	AuthRpcTimeouts    rpccfg.HTTPTimeouts
	EvmCallTimeout     time.Duration

	BatchLimit int // Maximum number of requests in a batch

	RPCSlowLogThreshold time.Duration
}
