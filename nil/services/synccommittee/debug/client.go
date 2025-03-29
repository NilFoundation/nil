package debug

import (
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

func NewClient(endpoint string, logger logging.Logger) public.TaskDebugApi {
	return rpc.NewTaskDebugRpcClient(endpoint, logger)
}
