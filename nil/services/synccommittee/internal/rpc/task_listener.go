package rpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type TaskListenerConfig struct {
	HttpEndpoint string
}

type TaskListener struct {
	config    *TaskListenerConfig
	scheduler scheduler.TaskScheduler
	logger    logging.Logger
}

func NewTaskListener(
	config *TaskListenerConfig,
	scheduler scheduler.TaskScheduler,
	logger logging.Logger,
) *TaskListener {
	listener := &TaskListener{
		config:    config,
		scheduler: scheduler,
	}

	listener.logger = srv.WorkerLogger(logger, listener)
	return listener
}

func (*TaskListener) Name() string {
	return "task_listener"
}

func (l *TaskListener) Run(context context.Context, started chan<- struct{}) error {
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
	return rpc.StartRpcServer(context, httpConfig, apiList, l.logger, started)
}
