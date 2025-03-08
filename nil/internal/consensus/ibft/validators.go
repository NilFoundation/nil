package ibft

import (
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
)

func (i *backendIBFT) calcProposer(height, round uint64, prevValidator *uint64) (*config.ValidatorInfo, uint64, error) {
	params, err := config.GetConfigParams(i.ctx, i.txFabric, i.shardId, height)
	if err != nil {
		i.logger.Error().
			Err(err).
			Uint64(logging.FieldRound, round).
			Uint64(logging.FieldHeight, height).
			Msg("Failed to get validators' params")
		return nil, 0, err
	}

	var seed uint64
	if prevValidator == nil {
		seed = round
	} else {
		seed = *prevValidator + round + 1
	}

	index := seed % uint64(len(params.ValidatorInfo))
	return &params.ValidatorInfo[index], index, nil
}
