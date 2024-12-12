package debug

import (
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/rs/zerolog"
)

func NewClient(endpoint string, logger zerolog.Logger) public.TaskDebugApi {
	return rpc.NewTaskDebugRpcClient(endpoint, logger)
}
