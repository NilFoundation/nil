package debug

import (
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

func NewTasksClient(endpoint string, logger logging.Logger) public.TaskDebugApi {
	return rpc.NewTaskDebugRpcClient(endpoint, logger)
}

func NewBlocksClient(endpoint string, logger logging.Logger) public.BlockDebugApi {
	return rpc.NewBlockDebugRpcClient(endpoint, logger)
}
