package tracer

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
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

	tracer RemoteTracer

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

func (s *TracerNildTestSuite) SetupSuite() {
	nilserviceCfg := &nilservice.Config{
		NShards:              3,
		HttpUrl:              rpctest.GetSockPath(s.T()),
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	}

	s.Start(nilserviceCfg)
	s.waitTwoBlocks()

	var err error
	s.tracer, err = NewRemoteTracer(s.Client, logging.NewLogger("tracer-test"))
	s.Require().NoError(err)

	s.addrFrom = types.MainSmartAccountAddress
	s.shardId = types.BaseShardId
}

func (s *TracerNildTestSuite) TearDownSuite() {
	s.Cancel()
}

func (s *TracerNildTestSuite) TestCounterContract() {
	deployPayload := contracts.CounterDeployPayload(s.T())
	contractAddr := types.CreateAddress(s.shardId, deployPayload)

	s.Run("SmartAccountDeploy", func() {
		txHash, err := s.Client.SendTransactionViaSmartAccount(
			s.Context,
			s.addrFrom,
			types.Code{},
			types.Gas(100_000).ToValue(types.DefaultGasPrice),
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
		traces := NewExecutionTraces()
		err = s.tracer.GetBlockTraces(s.Context, traces, types.BaseShardId, blkRef)
		s.Require().NoError(err)
	})

	s.Run("ContractDeploy", func() {
		// Deploy counter
		txHash, addr, err := s.Client.DeployContract(s.Context, s.shardId, s.addrFrom, deployPayload, types.Value{}, execution.MainPrivateKey)
		s.Require().NoError(err)
		s.Require().Equal(contractAddr, addr)

		receipt := s.WaitIncludedInMain(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("Add", func() {
		// Add to countuer (state change)
		txHash, err := s.Client.SendTransactionViaSmartAccount(
			s.Context,
			types.MainSmartAccountAddress,
			contracts.NewCounterAddCallData(s.T(), 5),
			types.Gas(100_000).ToValue(types.DefaultGasPrice),
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
		traces := NewExecutionTraces()
		err = s.tracer.GetBlockTraces(s.Context, traces, contractAddr.ShardId(), blkRef)
		s.Require().NoError(err)
	})

	s.Run("AllBlocksSerialization", func() {
		s.checkAllBlocksTracesSerialization()
	})
}

func (s *TracerNildTestSuite) TestTestContract() {
	deployPayload := contracts.GetDeployPayload(s.T(), contracts.NameTest)
	contractAddr := types.CreateAddress(s.shardId, deployPayload)

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
			types.Gas(100_000).ToValue(types.DefaultGasPrice),
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
		traces := NewExecutionTraces()
		err = s.tracer.GetBlockTraces(s.Context, traces, types.BaseShardId, blkRef)
		s.Require().NoError(err)
	})

	s.Run("ContractDeploy", func() {
		txHash, addr, err := s.Client.DeployContract(s.Context, s.shardId, s.addrFrom, deployPayload, types.Value{}, execution.MainPrivateKey)
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
			types.Gas(100_000).ToValue(types.DefaultGasPrice),
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
		traces := NewExecutionTraces()
		err = s.tracer.GetBlockTraces(s.Context, traces, contractAddr.ShardId(), blkRef)
		s.Require().NoError(err)
	})

	s.Run("AllBlocksSerialization", func() {
		s.checkAllBlocksTracesSerialization()
	})
}

// It looks like even smart account deploy is handled in multiple blocks, I don't know how to catch specific one for
// checks. Just prove every one.
func (s *TracerNildTestSuite) checkAllBlocksTracesSerialization() {
	for shardN := range s.ShardsNum {
		shardId := types.ShardId(shardN)
		latestBlock, err := s.Client.GetBlock(s.Context, shardId, "latest", false)
		s.Require().NoError(err)
		for blockNum := range latestBlock.Number {
			blkRef := transport.BlockNumber(blockNum).AsBlockReference()
			s.Require().NoError(err)
			blockTraces := NewExecutionTraces()
			err := s.tracer.GetBlockTraces(s.Context, blockTraces, shardId, blkRef)
			if errors.Is(err, ErrCantProofGenesisBlock) {
				continue
			}

			s.Require().NoError(err)

			tracesData, ok := blockTraces.(*executionTracesImpl)
			s.Require().True(ok)
			for _, cpEvt := range tracesData.CopyEvents {
				s.NotEmpty(cpEvt.Data)
			}

			// Test serialization
			tmpfile, err := os.CreateTemp("", "serialized_trace-")
			if err != nil {
				s.Require().NoError(err)
			}
			defer os.Remove(tmpfile.Name())

			err = SerializeToFile(blockTraces, MarshalModeBinary, tmpfile.Name())
			s.Require().NoError(err)
			deserializedTraces, err := DeserializeFromFile(tmpfile.Name(), MarshalModeBinary)
			s.Require().NoError(err)

			// Check if no data was lost after deserialization
			deserializedData, ok := deserializedTraces.(*executionTracesImpl)
			s.Require().True(ok)
			s.Require().Equal(len(deserializedData.StackOps), len(tracesData.StackOps))
			s.Require().Equal(len(deserializedData.MemoryOps), len(tracesData.MemoryOps))
			s.Require().Equal(len(deserializedData.StorageOps), len(tracesData.StorageOps))
			s.Require().Equal(len(deserializedData.ContractsBytecode), len(tracesData.ContractsBytecode))
			s.Require().Equal(len(deserializedData.CopyEvents), len(tracesData.CopyEvents))
			s.Require().Equal(len(deserializedData.ZKEVMStates), len(tracesData.ZKEVMStates))
			if tracesData.MPTTraces != nil {
				s.Require().Equal(len(deserializedData.MPTTraces.StorageTracesByAccount), len(tracesData.MPTTraces.StorageTracesByAccount))
				s.Require().Equal(len(deserializedData.MPTTraces.ContractTrieTraces), len(tracesData.MPTTraces.ContractTrieTraces))
			}
		}
	}
}
