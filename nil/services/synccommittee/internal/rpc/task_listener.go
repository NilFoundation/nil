package rpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/rs/zerolog"
)

type TaskListenerConfig struct {
	HttpEndpoint string
}

type TaskListener struct {
	config    *TaskListenerConfig
	scheduler scheduler.TaskScheduler
	logger    zerolog.Logger
}

func NewTaskListener(
	config *TaskListenerConfig,
	scheduler scheduler.TaskScheduler,
	logger zerolog.Logger,
) *TaskListener {
	return &TaskListener{
		config:    config,
		scheduler: scheduler,
		logger:    logger,
	}
}

func (l *TaskListener) Run(context context.Context) error {
	httpConfig := &httpcfg.HttpCfg{
		HttpURL:         l.config.HttpEndpoint,
		HttpCompression: true,
		TraceRequests:   true,
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
	}

	apiList := []transport.API{
		{
			Namespace: api.TaskRequestHandlerNamespace,
			Public:    true,
			Service:   api.TaskRequestHandler(l.scheduler),
			Version:   "1.0",
		},
		{
			Namespace: public.DebugNamespace,
			Public:    true,
			Service:   public.TaskDebugApi(l.scheduler),
			Version:   "1.0",
		},
	}

	l.logger.Info().Msgf("Open task listener endpoint %v", l.config.HttpEndpoint)
	return rpc.StartRpcServer(context, httpConfig, apiList, l.logger)
}
