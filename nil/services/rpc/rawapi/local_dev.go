package rawapi

import (
	"context"
	"errors"
	"time"
)

func (api *LocalShardApi) DoPanicOnShard(ctx context.Context) (uint64, error) {
	if !api.enableDevApi {
		return 0, errors.New("dev api is not enabled")
	}
	go func() {
		time.Sleep(10 * time.Second)
		panic("RPC request for panic on shard")
	}()
	return 0, nil
}
