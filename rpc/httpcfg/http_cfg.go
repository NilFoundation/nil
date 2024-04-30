package httpcfg

import (
	"time"

	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
)

type HttpCfg struct {
	Enabled bool

	AuthRpcHTTPListenAddress string

	HttpServerEnabled  bool
	HttpURL            string
	HttpListenAddress  string
	HttpPort           int
	HttpCORSDomain     []string
	HttpVirtualHost    []string
	AuthRpcVirtualHost []string
	HttpCompression    bool

	AuthRpcPort    int
	PrivateApiAddr string

	API                  []string
	Gascap               uint64
	MaxTraces            uint64
	RpcAllowListFilePath string
	RpcBatchConcurrency  uint
	DBReadConcurrency    int
	TxPoolApiAddr        string

	JWTSecretPath             string // Engine API Authentication
	TraceRequests             bool   // Print requests to logs at INFO level
	DebugSingleRequest        bool   // Print single-request-related debugging info to logs at INFO level
	HTTPTimeouts              rpccfg.HTTPTimeouts
	AuthRpcTimeouts           rpccfg.HTTPTimeouts
	EvmCallTimeout            time.Duration
	OverlayGetLogsTimeout     time.Duration
	OverlayReplayBlockTimeout time.Duration

	BatchLimit                  int  // Maximum number of requests in a batch
	ReturnDataLimit             int  // Maximum number of bytes returned from calls (like eth_call)
	AllowUnprotectedTxs         bool // Whether to allow non EIP-155 protected transactions  txs over RPC
	MaxGetProofRewindBlockCount int  //Max GetProof rewind block count

	RPCSlowLogThreshold time.Duration
}
