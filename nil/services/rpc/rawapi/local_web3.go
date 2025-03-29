package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/version"
)

func (api *LocalShardApi) ClientVersion(ctx context.Context) (string, error) {
	return version.BuildClientVersion("=;Nil"), nil
}
