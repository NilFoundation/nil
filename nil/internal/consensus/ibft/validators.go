package ibft

import (
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
)

func (i *backendIBFT) calcProposer(height, round uint64) (*config.ValidatorInfo, error) {
	params, err := i.getConfigParams(i.ctx, height)
	if err != nil {
		i.logger.Error().
			Err(err).
			Uint64(logging.FieldRound, round).
			Uint64(logging.FieldHeight, height).
			Msg("Failed to get validators' config params")
		return nil, err
	}
	validators := params.ValidatorInfo

	index := (height + round) % uint64(len(validators))
	return &validators[index], nil
}
