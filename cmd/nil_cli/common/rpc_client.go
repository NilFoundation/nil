package common

import (
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/version"
	"github.com/rs/zerolog"
)

var client *rpc.Client

func InitRpcClient(cfg *Config, logger zerolog.Logger) {
	client = rpc.NewClientWithDefaultHeaders(
		cfg.RPCEndpoint,
		logger,
		map[string]string{
			"User-Agent": "nil-cli/" + version.GetGitRevision(),
		},
	)
}

func GetRpcClient() *rpc.Client {
	check.PanicIfNot(client != nil)
	return client
}
