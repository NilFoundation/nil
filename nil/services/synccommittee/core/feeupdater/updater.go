package feeupdater

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/fetching"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/jonboulle/clockwork"
)

type Config struct {
	// Max relative diff which triggers update on L1 chain
	MaxFeePerGasUpdateThreshold float64 `yaml:"maxFeePerGasUpdateThreshold"`

	// Max interval between L2 shards polling
	PollInterval time.Duration `yaml:"feeUpdaterPollInterval"`

	// Max possible time without L1 chain update
	MaxUpdateInterval time.Duration `yaml:"feeUpdaterMaxUpdateInterval"`

	// Statically configured priority fee per gas value
	MaxPriorityFeePerGasFixed int64 `yaml:"maxPriorityFeePerGas"`

	// Additional coefficient added to the max fee fetched from cluster
	MarkupPercent uint `yaml:"markupPercent"`

	// Minimum expected value of fee per gas collected from shards
	MinFeePerGas uint64 `yaml:"minFeePerGas"`
}

func DefaultConfig() Config {
	return Config{
		MaxFeePerGasUpdateThreshold: 0.1,
		PollInterval:                time.Minute,
		MaxUpdateInterval:           2 * time.Hour,
		MarkupPercent:               25,
		MaxPriorityFeePerGasFixed:   1_000_000,  // 0.001 gwei transformed into wei
		MinFeePerGas:                20_000_000, // 0.02 gwei transformed into wei
	}
}

type feeParams struct {
	maxFeePerGas         *big.Int
	maxPriorityFeePerGas *big.Int
}

type Updater struct {
	config          Config
	blockFetcher    *fetching.Fetcher
	logger          logging.Logger
	clock           clockwork.Clock
	contractBinding NilGasPriceOracleContract
	metrics         *metrics.FeeUpdaterMetrics

	state struct {
		feeParams            *feeParams
		lastUpdatedTimestamp time.Time
	}
}

func NewUpdater(
	config Config,
	blockFetcher *fetching.Fetcher,
	logger logging.Logger,
	clock clockwork.Clock,
	contractBinding NilGasPriceOracleContract,
	metrics *metrics.FeeUpdaterMetrics,
) *Updater {
	return &Updater{
		config:          config,
		blockFetcher:    blockFetcher,
		logger:          logger,
		clock:           clock,
		contractBinding: contractBinding,
		metrics:         metrics,
	}
}

func (u *Updater) Name() string {
	return "l1_fee_updater"
}

func (u *Updater) Run(ctx context.Context, started chan<- struct{}) error {
	return u.runUpdater(ctx, started)
}

func (u *Updater) runUpdater(ctx context.Context, started chan<- struct{}) error {
	ticker := u.clock.NewTicker(u.config.PollInterval)
	close(started)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.Chan():
			u.logger.Debug().Msg("fetcher wake up by timer")
			if err := u.recalcBaseFee(ctx); err != nil {
				u.logger.Error().Err(err).Msg("failed to recalculate L2 fee params")
				u.metrics.RecordError(ctx, u.Name())
			}
		}
	}
}

func (u *Updater) recalcBaseFee(ctx context.Context) error {
	shardList, err := u.blockFetcher.GetShardIdList(ctx)
	if err != nil {
		return err
	}

	var (
		maxBaseFee          types.Value
		shardWithMaxBaseFee types.ShardId
	)
	for _, shard := range shardList {
		block, err := u.blockFetcher.GetLatestBlock(ctx, shard)
		if err != nil {
			return err
		}

		if block.BaseFee.Cmp(maxBaseFee) > 0 {
			maxBaseFee = block.BaseFee
			shardWithMaxBaseFee = shard
		}
	}
	u.logger.Debug().
		Uint32("shard_id", uint32(shardWithMaxBaseFee)).
		Uint256("max_current_base_fee", maxBaseFee.String()).
		Msg("fetched new value of max base fee")

	maxBaseFee = maxBaseFee.Mul64(uint64(100 + u.config.MarkupPercent)).Div64(100)

	if maxBaseFee.Int().CmpUint64(u.config.MinFeePerGas) < 0 {
		return fmt.Errorf("too small maxFeePerGas value evaluated: %s", maxBaseFee)
	}

	update := feeParams{
		maxFeePerGas:         maxBaseFee.ToBig(),
		maxPriorityFeePerGas: big.NewInt(u.config.MaxPriorityFeePerGasFixed),
	}

	u.logger.Info().
		Stringer("max_fee_per_gas", update.maxFeePerGas).
		Stringer("max_priority_fee_per_gas", update.maxPriorityFeePerGas).
		Msg("evaluated new fee params")

	return u.applyUpdate(ctx, update)
}

func (u *Updater) applyUpdate(ctx context.Context, update feeParams) error {
	var updateL1 bool
	for _, condition := range u.getUpdateConditions() {
		if condition.check(&update) {
			u.logger.Info().Str("reason", condition.reason).Msg("L1 data update requested")
			updateL1 = true
			break
		}
	}

	if !updateL1 {
		u.logger.Info().Msg("L1 data update is not requested")
		return nil
	}

	now := u.clock.Now()
	if err := u.contractBinding.SetOracleFee(ctx, update); err != nil {
		u.logger.Error().Err(err).Msg("failed to update L1 state")
		return err
	}

	// Do we need to preseve these values between service restarts?
	u.state.feeParams = &update
	u.state.lastUpdatedTimestamp = now

	u.logger.Info().Msg("L1 fee data updated")
	u.metrics.RegisterL1Update(ctx)

	return nil
}

func (u *Updater) isFirstUpdate(_ *feeParams) bool {
	return u.state.feeParams == nil
}

func (u *Updater) isMaxUpdateIntervalElapsed(_ *feeParams) bool {
	now := u.clock.Now()
	diff := now.Sub(u.state.lastUpdatedTimestamp)
	u.logger.Debug().Dur("not_updated_for", diff).Msg("check if it is time to update")
	return diff >= u.config.MaxUpdateInterval
}

func (u *Updater) isChangeSignificant(update *feeParams) bool {
	diff := new(big.Int).Sub(
		update.maxFeePerGas,
		u.state.feeParams.maxFeePerGas,
	).Uint64()

	current, _ := u.state.feeParams.maxFeePerGas.Float64()

	relativeDiff := float64(diff) / current

	u.logger.Debug().
		Float64("relative_diff", relativeDiff).
		Msg("check if maxFeePerGas diff reached threshold")

	return relativeDiff >= u.config.MaxFeePerGasUpdateThreshold
}

type updateCondition struct {
	reason string
	check  func(*feeParams) bool
}

func (u *Updater) getUpdateConditions() []updateCondition {
	return []updateCondition{
		// Prioritized list of conditions, first match triggers update submission to L1
		{"first_update", u.isFirstUpdate},
		{"max_update_interval_elapsed", u.isMaxUpdateIntervalElapsed},
		{"relative_diff_threshold_reached", u.isChangeSignificant},
	}
}
