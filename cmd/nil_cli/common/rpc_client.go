package common

import (
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/version"
)

var client *rpc.Client

func InitRpcClient(cfg *Config) {
	client = rpc.NewClientWithDefaultHeaders(
		cfg.RPCEndpoint,
		map[string]string{
			"User-Agent": "nil-cli/" + version.GetGitRevision(),
		},
	)
}

func GetRpcClient() *rpc.Client {
	check.PanicIfNot(client != nil)
	return client
}
