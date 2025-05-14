package rpc

import (
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

func TaskRequestServerHandler(service api.TaskRequestHandler) Handler {
	return NewHandler(api.TaskRequestHandlerNamespace, service)
}

func DebugTasksServerHandler(service public.TaskDebugApi) Handler {
	return NewHandler(public.DebugTasksNamespace, service)
}

func DebugBlocksServerHandler(service public.BlockDebugApi) Handler {
	return NewHandler(public.DebugBlocksNamespace, service)
}

func NewServerWithTasks(
	config ServerConfig,
	logger logging.Logger,
	requestHandler api.TaskRequestHandler,
	debugApi public.TaskDebugApi,
	additionalHandlers ...Handler,
) *server {
	handlers := make([]Handler, 0, len(additionalHandlers)+2)
	handlers = append(handlers, TaskRequestServerHandler(requestHandler))
	handlers = append(handlers, DebugTasksServerHandler(debugApi))
	handlers = append(handlers, additionalHandlers...)
	return NewServer(config, logger, handlers...)
}
