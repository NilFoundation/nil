package listener

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/rs/zerolog"
)

type TaskListenerConfig struct {
	HttpEndpoint string
}

type TaskListener struct {
	config         *TaskListenerConfig
	requestHandler api.TaskRequestHandler
	logger         zerolog.Logger
}

func NewTaskListener(
	config *TaskListenerConfig,
	requestHandler api.TaskRequestHandler,
	logger zerolog.Logger,
) *TaskListener {
	return &TaskListener{
		config:         config,
		requestHandler: requestHandler,
		logger:         logger,
	}
}

func (l *TaskListener) Run(context context.Context) error {
	httpConfig := &httpcfg.HttpCfg{
		Enabled:         true,
		HttpURL:         l.config.HttpEndpoint,
		HttpCompression: true,
		TraceRequests:   true,
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
	}

	apiList := []transport.API{
		{
			Namespace: api.TaskRequestHandlerNamespace,
			Public:    true,
			Service:   l.requestHandler,
			Version:   "1.0",
		},
	}

	return rpc.StartRpcServer(context, httpConfig, apiList, l.logger)
}
