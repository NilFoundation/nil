package rpc

import (
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

func NewServerWithTasks(
	config ServerConfig,
	logger logging.Logger,
	requestHandler api.TaskRequestHandler,
	debugApi public.TaskDebugApi,
	additionalHandlers ...Handler,
) *server {
	handlers := make([]Handler, 0, len(additionalHandlers)+2)
	handlers = append(handlers, NewHandler(api.TaskRequestHandlerNamespace, requestHandler))
	handlers = append(handlers, NewHandler(public.DebugNamespace, debugApi))
	handlers = append(handlers, additionalHandlers...)
	return NewServer(config, logger, handlers...)
}
