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
	config       *TaskListenerConfig
	taskObserver api.TaskObserver
	logger       zerolog.Logger
}

func NewTaskListener(
	config *TaskListenerConfig,
	taskObserver api.TaskObserver,
	logger zerolog.Logger,
) *TaskListener {
	return &TaskListener{
		config:       config,
		taskObserver: taskObserver,
		logger:       logger,
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
			Namespace: api.TaskObserverNamespace,
			Public:    true,
			Service:   l.taskObserver,
			Version:   "1.0",
		},
	}

	return rpc.StartRpcServer(context, httpConfig, apiList, l.logger)
}
