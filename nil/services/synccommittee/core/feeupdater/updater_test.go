package feeupdater

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/fetching"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type UpdaterTestSuite struct {
	suite.Suite

	updater *Updater
	config  Config
	metrics *metrics.FeeUpdaterMetrics

	rpcClientMock  client.ClientMock
	l1ContractMock *NilGasPriceOracleContractMock
	clock          *clockwork.FakeClock

	ctx            context.Context
	cancel         context.CancelFunc
	updaterStopped chan struct{}
}

func TestUpdater(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(UpdaterTestSuite))
}

func (s *UpdaterTestSuite) SetupTest() {
	metrics, err := metrics.NewFeeUpdaterMetrics()
	s.Require().NoError(err)

	s.metrics = metrics
	s.rpcClientMock = client.ClientMock{}
	s.l1ContractMock = &NilGasPriceOracleContractMock{}
	s.clock = clockwork.NewFakeClockAt(time.Unix(0, 0))
	logger := logging.NewLogger("fee_updater_test")

	s.rpcClientMock.GetShardIdListFunc = func(_ context.Context) ([]types.ShardId, error) {
		shards := []types.ShardId{0, 1, 2, 3}
		return shards, nil
	}

	fetcher := fetching.NewFetcher(&s.rpcClientMock, logger)

	s.config = DefaultConfig()

	s.updater = NewUpdater(
		s.config,
		fetcher,
		logger,
		s.clock,
		s.l1ContractMock,
		s.metrics,
	)

	s.ctx, s.cancel = context.WithCancel(s.T().Context())
}

func (s *UpdaterTestSuite) runUpdater() {
	started := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		err := s.updater.Run(s.ctx, started)
		if err != nil {
			s.ErrorIs(err, context.Canceled)
		}
		close(stopped)
	}()
	<-started
	s.updaterStopped = stopped
}

func (s *UpdaterTestSuite) TearDownTest() {
	s.cancel()
	<-s.updaterStopped
}

func (s *UpdaterTestSuite) expectedMaxFeePerGas(maxValue uint64) uint64 {
	coeff := 100 + uint64(s.config.MarkupPercent)
	ret := maxValue * coeff
	ret /= 100
	return ret
}

func (s *UpdaterTestSuite) configureFeeData(initMaxVal uint64) *atomic.Uint64 {
	var capturedMaxVal atomic.Uint64
	capturedMaxVal.Store(initMaxVal)

	s.rpcClientMock.GetBlockFunc = func(
		_ context.Context,
		shardId types.ShardId,
		blockId any,
		fullTx bool,
	) (*jsonrpc.RPCBlock, error) {
		v := capturedMaxVal.Load()
		feeByShardMap := map[types.ShardId]uint64{
			0: v / 2,
			1: v,
			2: v / 4,
			3: v - 1,
		}
		s.Equal("latest", blockId)
		s.False(fullTx)

		block := &jsonrpc.RPCBlock{
			BaseFee: types.NewValueFromUint64(feeByShardMap[shardId]),
			Number:  1111,
			Hash:    common.EmptyHash,
		}

		return block, nil
	}

	return &capturedMaxVal
}

func (s *UpdaterTestSuite) TestInitialUpdate() {
	s.rpcClientMock.GetBlockFunc = func(
		_ context.Context,
		shardId types.ShardId,
		blockId any,
		fullTx bool,
	) (*jsonrpc.RPCBlock, error) {
		feeByShardMap := map[types.ShardId]uint64{
			0: 10_000_000,
			1: 200_000_000, // max value
			2: 150_000_000,
			3: 50_000_000,
		}
		s.Equal("latest", blockId)
		s.False(fullTx)

		block := &jsonrpc.RPCBlock{
			BaseFee: types.NewValueFromUint64(feeByShardMap[shardId]),
			Number:  1111,
			Hash:    common.EmptyHash,
		}

		return block, nil
	}

	s.runUpdater()

	updateCalled := make(chan struct{})
	entered := false
	s.l1ContractMock.SetOracleFeeFunc = func(
		_ context.Context,
		params feeParams,
	) error {
		s.Require().False(entered, "expected only one update")
		entered = true

		s.EqualValues(250_000_000, params.maxFeePerGas.Uint64())
		s.EqualValues(10_000_00, params.maxPriorityFeePerGas.Uint64())
		close(updateCalled)
		return nil
	}

	const ticks = 3

	s.clock.Advance(ticks * s.config.PollInterval)
	for range ticks {
		<-updateCalled
	}
}

func (s *UpdaterTestSuite) TestUpdateOnSignificantChange() {
	maxValue := s.configureFeeData(200_000_000)

	updateCalled := make(chan struct{})
	s.l1ContractMock.SetOracleFeeFunc = func(
		_ context.Context,
		params feeParams,
	) error {
		s.Equal(s.expectedMaxFeePerGas(maxValue.Load()), params.maxFeePerGas.Uint64())
		s.EqualValues(10_000_00, params.maxPriorityFeePerGas.Uint64())
		updateCalled <- struct{}{}
		return nil
	}

	s.runUpdater()

	<-updateCalled // first update

	maxValue.Store(maxValue.Load() * 2)
	s.clock.Advance(s.config.PollInterval)
	<-updateCalled // significant change update
}

func (s *UpdaterTestSuite) TestUpdateOnTimeInterval() {
	maxValue := s.configureFeeData(200_000_000)

	updateCalled := make(chan struct{})
	s.l1ContractMock.SetOracleFeeFunc = func(
		_ context.Context,
		params feeParams,
	) error {
		s.Equal(s.expectedMaxFeePerGas(maxValue.Load()), params.maxFeePerGas.Uint64())
		s.EqualValues(10_000_00, params.maxPriorityFeePerGas.Uint64())
		updateCalled <- struct{}{}
		return nil
	}

	s.runUpdater()

	s.clock.Advance(s.config.PollInterval)
	<-updateCalled // first update

	// not a significant change
	maxValue.Add(uint64(float64(maxValue.Load()) * (s.config.MaxFeePerGasUpdateThreshold / 2)))
	s.clock.Advance(s.config.MaxUpdateInterval + time.Second)
	<-updateCalled // update after configured time interval
}

func (s *UpdaterTestSuite) TestNoRedundantUpdates() {
	maxValue := s.configureFeeData(200_000_000)

	initialzed := make(chan struct{})
	s.l1ContractMock.SetOracleFeeFunc = func(
		_ context.Context,
		params feeParams,
	) error {
		s.Equal(s.expectedMaxFeePerGas(maxValue.Load()), params.maxFeePerGas.Uint64())
		s.EqualValues(10_000_00, params.maxPriorityFeePerGas.Uint64())
		close(initialzed)
		return nil
	}

	s.runUpdater()

	s.clock.Advance(s.config.PollInterval)
	<-initialzed // first update

	updated := make(chan struct{})
	s.l1ContractMock.SetOracleFeeFunc = func(
		_ context.Context,
		params feeParams,
	) error {
		s.Equal(s.expectedMaxFeePerGas(maxValue.Load()), params.maxFeePerGas.Uint64())
		s.EqualValues(10_000_00, params.maxPriorityFeePerGas.Uint64())
		close(updated)
		return nil
	}

	const iterations = 10
	toAdd := (float64(maxValue.Load()) * s.config.MaxFeePerGasUpdateThreshold) / iterations

	for range iterations {
		s.clock.Advance(s.config.PollInterval)
		select {
		case <-updated:
			s.Fail("not expected to be called now")
		default:
		}
		maxValue.Add(uint64(toAdd))
	}

	<-updated // update only once after significant change was reached
}
