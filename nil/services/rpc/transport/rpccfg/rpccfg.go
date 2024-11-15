package rpccfg

import (
	"time"
)

const (
	DefaultEvmCallTimeout            = 5 * time.Minute
	DefaultOverlayGetLogsTimeout     = 5 * time.Minute
	DefaultOverlayReplayBlockTimeout = 10 * time.Second
)

var ContentType = "application/json"

// https://www.jsonrpc.org/historical/json-rpc-over-http.html#id13
var AcceptedContentTypes = []string{ContentType, "application/json-rpc", "application/jsonrequest"}

var SlowLogBlackList = []string{
	"eth_getBlock", "eth_getBlockByNumber", "eth_getBlockByHash", "eth_blockNumber",
	"eth_call", "eth_getInMessageByHash",
}

var HeavyLogMethods = map[string]struct{}{
	"cometa_registerContract": {},
	"eth_call":                {},
	"eth_estimateGas":         {},
	"eth_sendRawTransaction":  {},
}
