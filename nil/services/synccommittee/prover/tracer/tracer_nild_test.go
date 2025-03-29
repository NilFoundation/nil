package tracer

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	rpctest "github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type TracerNildTestSuite struct {
	tests.RpcSuite

	addrFrom types.Address
	shardId  types.ShardId
}

func TestTracerNildTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(TracerNildTestSuite))
}

func (s *TracerNildTestSuite) waitTwoBlocks() {
	s.T().Helper()
	const (
		zeroStateWaitTimeout  = 5 * time.Second
		zeroStatePollInterval = time.Second
	)
	for i := range s.ShardsNum {
		s.Require().Eventually(func() bool {
			block, err := s.Client.GetBlock(s.Context, types.ShardId(i), transport.BlockNumber(1), false)
			return err == nil && block != nil
		}, zeroStateWaitTimeout, zeroStatePollInterval)
	}
}

func (s *TracerNildTestSuite) initTracer() RemoteTracesCollector {
	s.T().Helper()
	var err error
	tracer, err := NewRemoteTracesCollector(s.Context, s.Client, logging.NewLogger("tracer-test"))
	s.Require().NoError(err)
	return tracer
}

func (s *TracerNildTestSuite) getSingleBlockTraces(
	shardId types.ShardId, blockRef transport.BlockReference,
) *ExecutionTraces {
	s.T().Helper()
	var err error
	tracer := s.initTracer()
	traces, err := tracer.GetBlockTraces(s.Context, BlockId{shardId, blockRef})
	s.Require().NoError(err)
	return traces
}

func (s *TracerNildTestSuite) SetupSuite() {
	nilserviceCfg := &nilservice.Config{
		NShards:              3,
		HttpUrl:              rpctest.GetSockPath(s.T()),
		CollatorTickPeriodMs: 400,
		DisableConsensus:     true,
	}

	s.Start(nilserviceCfg)
	s.waitTwoBlocks()

	s.addrFrom = types.MainSmartAccountAddress
	s.shardId = types.BaseShardId
}

func (s *TracerNildTestSuite) TearDownSuite() {
	s.Cancel()
}

func (s *TracerNildTestSuite) TestCounterContract() {
	deployPayload := contracts.CounterDeployPayload(s.T())
	contractAddr := types.CreateAddress(s.shardId, deployPayload)
	latestBlocks := s.getLatestBlocksForShards()

	s.Run("SmartAccountDeploy", func() {
		txHash, err := s.Client.SendTransactionViaSmartAccount(
			s.Context,
			s.addrFrom,
			types.Code{},
			types.NewFeePackFromGas(100_000),
			types.NewValueFromUint64(1337),
			[]types.TokenBalance{},
			contractAddr,
			execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)
		blkRef := transport.BlockNumber(receipt.BlockNumber).AsBlockReference()
		_ = s.getSingleBlockTraces(types.BaseShardId, blkRef)
	})

	s.Run("ContractDeploy", func() {
		// Deploy counter
		txHash, addr, err := s.Client.DeployContract(
			s.Context, s.shardId, s.addrFrom, deployPayload, types.Value{}, types.NewFeePackFromGas(300_000),
			execution.MainPrivateKey)
		s.Require().NoError(err)
		s.Require().Equal(contractAddr, addr)

		receipt := s.WaitIncludedInMain(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)
		s.Require().True(receipt.OutReceipts[0].Success)

		// TODO: why this fails?
		// _ = s.getSingleBlockTraces(s.shardId, transport.HashBlockReference(receipt.BlockHash))
	})

	s.Run("Add", func() {
		// Add to counter (state change)
		txHash, err := s.Client.SendTransactionViaSmartAccount(
			s.Context,
			types.MainSmartAccountAddress,
			contracts.NewCounterAddCallData(s.T(), 5),
			types.NewFeePackFromGas(100_000),
			types.NewZeroValue(),
			[]types.TokenBalance{},
			contractAddr,
			execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.WaitIncludedInMain(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)
		s.Require().True(receipt.OutReceipts[0].Success)

		blkRef := transport.BlockNumber(receipt.OutReceipts[0].BlockNumber).AsBlockReference()
		_ = s.getSingleBlockTraces(contractAddr.ShardId(), blkRef)
	})

	s.Run("AllBlocksSerialization", func() {
		s.checkBlocksRangeTracesSerialization(latestBlocks, false)
		s.checkBlocksRangeTracesSerialization(latestBlocks, true)
	})
}

func (s *TracerNildTestSuite) TestTestContract() {
	deployPayload := contracts.GetDeployPayload(s.T(), contracts.NameTest)
	contractAddr := types.CreateAddress(s.shardId, deployPayload)
	latestBlocks := s.getLatestBlocksForShards()

	testAddresses := make(map[types.ShardId]types.Address)
	for shardN := range s.ShardsNum {
		shardId := types.ShardId(shardN)
		addr, err := contracts.CalculateAddress(contracts.NameTest, shardId, []byte{byte(shardN)})
		s.Require().NoError(err)
		testAddresses[shardId] = addr
	}

	s.Run("SmartAccountDeploy", func() {
		txHash, err := s.Client.SendTransactionViaSmartAccount(
			s.Context,
			s.addrFrom,
			types.Code{},
			types.NewFeePackFromGas(100_000),
			types.NewValueFromUint64(1337),
			[]types.TokenBalance{},
			contractAddr,
			execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)
		blkRef := transport.BlockNumber(receipt.BlockNumber).AsBlockReference()
		_ = s.getSingleBlockTraces(types.BaseShardId, blkRef)
	})

	s.Run("ContractDeploy", func() {
		txHash, addr, err := s.Client.DeployContract(
			s.Context, s.shardId, s.addrFrom, deployPayload, types.Value{}, types.NewFeePackFromGas(3_000_000),
			execution.MainPrivateKey)
		s.Require().NoError(err)
		s.Require().Equal(contractAddr, addr)

		receipt := s.WaitIncludedInMain(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)
	})

	s.Run("EmitEvent", func() {
		callData := contracts.NewCallDataT(
			s.T(),
			contracts.NameTest,
			"emitEvent",
			types.NewValueFromUint64(1),
			types.NewValueFromUint64(2),
		)
		txHash, err := s.Client.SendTransactionViaSmartAccount(
			s.Context,
			types.MainSmartAccountAddress,
			callData,
			types.NewFeePackFromGas(100_000),
			types.NewZeroValue(),
			[]types.TokenBalance{},
			contractAddr,
			execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.WaitIncludedInMain(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)
		s.Require().True(receipt.OutReceipts[0].Success)

		blkRef := transport.BlockNumber(receipt.BlockNumber).AsBlockReference()
		_ = s.getSingleBlockTraces(contractAddr.ShardId(), blkRef)
	})

	s.Run("AllBlocksSerialization", func() {
		s.checkBlocksRangeTracesSerialization(latestBlocks, false)
		s.checkBlocksRangeTracesSerialization(latestBlocks, true)
	})
}

func (s *TracerNildTestSuite) getLatestBlocksForShards() []types.BlockNumber {
	s.T().Helper()
	latestBlocksForShards := make([]types.BlockNumber, s.ShardsNum)
	for shardId := range s.ShardsNum {
		latestBlock, err := s.Client.GetBlock(s.Context, types.ShardId(shardId), "latest", false)
		s.Require().NoError(err)
		latestBlocksForShards[shardId] = latestBlock.Number
	}
	return latestBlocksForShards
}

func (s *TracerNildTestSuite) checkTracesSerialization(traces *ExecutionTraces) {
	s.T().Helper()
	tmpfile, err := os.CreateTemp("", "serialized_trace-")
	if err != nil {
		s.Require().NoError(err)
	}
	defer os.Remove(tmpfile.Name())

	err = SerializeToFile(traces, MarshalModeBinary, tmpfile.Name())
	s.Require().NoError(err)
	deserializedTraces, err := DeserializeFromFile(tmpfile.Name(), MarshalModeBinary)
	s.Require().NoError(err)

	// Check if no data was lost after deserialization

	s.Require().Equal(len(deserializedTraces.StackOps), len(traces.StackOps))
	s.Require().Equal(len(deserializedTraces.MemoryOps), len(traces.MemoryOps))
	s.Require().Equal(len(deserializedTraces.StorageOps), len(traces.StorageOps))
	s.Require().Equal(len(deserializedTraces.ContractsBytecode), len(traces.ContractsBytecode))
	s.Require().Equal(len(deserializedTraces.CopyEvents), len(traces.CopyEvents))
	s.Require().Equal(len(deserializedTraces.ZKEVMStates), len(traces.ZKEVMStates))
}

// Even smart account deploy is handled in multiple blocks, trace last N blocks for each shard to include
// all produced transactions. If `multiBlock` is true, traces will be collected from range of blocks, otherwise,
// tracer will be called for each block individually.
func (s *TracerNildTestSuite) checkBlocksRangeTracesSerialization(from []types.BlockNumber, multiBlock bool) {
	latestBlocksForShards := s.getLatestBlocksForShards()
	for shardId, latestBlockNum := range latestBlocksForShards {
		if multiBlock {
			blockIds := make([]BlockId, 0, latestBlockNum-from[shardId]+1)
			for blockNum := from[shardId]; blockNum <= latestBlockNum; blockNum++ {
				blockIds = append(
					blockIds, BlockId{types.ShardId(shardId), transport.Uint64BlockReference(blockNum.Uint64())},
				)
			}
			traces, err := CollectTraces(s.Context, s.Client, &TraceConfig{
				BlockIDs: blockIds,
			})
			s.Require().NoError(err)

			s.checkTracesSerialization(traces)
		} else {
			for blockNum := from[shardId]; blockNum <= latestBlockNum; blockNum++ {
				tracer := s.initTracer()
				blkRef := transport.BlockNumber(blockNum).AsBlockReference()
				traces, err := tracer.GetBlockTraces(s.Context, BlockId{types.ShardId(shardId), blkRef})
				if errors.Is(err, ErrCantProofGenesisBlock) {
					continue
				}
				mptTraces, err := tracer.GetMPTTraces()
				s.Require().NoError(err)
				traces.SetMptTraces(&mptTraces)

				s.checkTracesSerialization(traces)
			}
		}
	}
}
