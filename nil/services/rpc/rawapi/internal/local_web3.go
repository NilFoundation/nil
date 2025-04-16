package internal

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/version"
)

func (api *localShardApiRo) ClientVersion(ctx context.Context) (string, error) {
	return version.BuildClientVersion("=;Nil"), nil
}
