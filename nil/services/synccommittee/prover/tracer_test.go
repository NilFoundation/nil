package prover

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	niltypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	rpctest "github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/stretchr/testify/suite"
)

type TracerTestSuite struct {
	suite.Suite

	context      context.Context
	cancellation context.CancelFunc
	tracer       RemoteTracer
	nilCancel    context.CancelFunc
	doneChan     chan interface{} // to track when nilservice has finished
	nilDb        db.DB
	nShards      uint32
	rpcClient    *rpc.Client

	addrFrom types.Address
	shardId  types.ShardId
}

func (s *TracerTestSuite) waitTwoBlocks(endpoint string) {
	s.T().Helper()
	logger := logging.NewLogger("proofprovider-test")
	// client := rpc.NewClient(endpoint, zerolog.Nop())
	client := rpc.NewClient(endpoint, logger)
	const (
		zeroStateWaitTimeout  = 5 * time.Second
		zeroStatePollInterval = time.Second
	)
	for i := range s.nShards {
		s.Require().Eventually(func() bool {
			block, err := client.GetBlock(niltypes.ShardId(i), transport.BlockNumber(1), false)
			return err == nil && block != nil
		}, zeroStateWaitTimeout, zeroStatePollInterval)
	}
}

func (s *TracerTestSuite) setupNild() string {
	s.T().Helper()

	s.nShards = 5

	url := rpctest.GetSockPath(s.T())
	var err error
	s.nilDb, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	nilserviceCfg := &nilservice.Config{
		NShards:              s.nShards,
		HttpUrl:              url,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	}
	var nilContext context.Context
	nilContext, s.nilCancel = context.WithCancel(context.Background())
	s.doneChan = make(chan interface{})
	go func() {
		nilservice.Run(nilContext, nilserviceCfg, s.nilDb, nil)
		s.doneChan <- nil
	}()
	s.waitTwoBlocks(url)
	return url
}

const (
	ReceiptWaitTimeout    = 15 * time.Second
	ReceiptPollInterval   = 250 * time.Millisecond
	ZeroStateWaitTimeout  = 10 * time.Second
	ZeroStatePollInterval = 100 * time.Millisecond
)

func (s *TracerTestSuite) waitForReceiptCommon(hash common.Hash, check func(*jsonrpc.RPCReceipt) bool) *jsonrpc.RPCReceipt {
	s.T().Helper()

	var receipt *jsonrpc.RPCReceipt
	var err error
	s.Require().Eventually(func() bool {
		receipt, err = s.rpcClient.GetInMessageReceipt(hash)
		s.Require().NoError(err)
		return check(receipt)
	}, ReceiptWaitTimeout, ReceiptPollInterval)

	s.Equal(hash, receipt.MsgHash)

	return receipt
}

func (s *TracerTestSuite) waitForReceipt(hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return s.waitForReceiptCommon(hash, func(receipt *jsonrpc.RPCReceipt) bool {
		return receipt.IsComplete()
	})
}

func (s *TracerTestSuite) waitIncludedInMain(hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return s.waitForReceiptCommon(hash, func(receipt *jsonrpc.RPCReceipt) bool {
		return receipt.IsCommitted()
	})
}

func (s *TracerTestSuite) SetupSuite() {
	s.context, s.cancellation = context.WithCancel(context.Background())

	rpcUrl := s.setupNild()

	var err error
	s.rpcClient = rpc.NewClient(rpcUrl, logging.NewLogger("client-test"))
	s.tracer, err = NewRemoteTracer(s.rpcClient, logging.NewLogger("tracer-test"))
	s.Require().NoError(err)

	s.addrFrom = types.MainWalletAddress
	s.shardId = types.BaseShardId
}

func (s *TracerTestSuite) TearDownSuite() {
	s.cancellation()
	s.nilCancel()
	<-s.doneChan // Wait for nilservice to shutdown
	s.nilDb.Close()
	// TODO: remove nilDb.Close() if it doesn't fails now or add one for s.db
}

func (s *TracerTestSuite) TestCounterContract() {
	ctx := context.Background()

	deployPayload := contracts.CounterDeployPayload(s.T())
	contractAddr := types.CreateAddress(s.shardId, deployPayload)

	s.Run("WalletDeploy", func() {
		txHash, err := s.rpcClient.SendMessageViaWallet(
			s.addrFrom,
			types.Code{},
			types.Gas(100_000).ToValue(types.DefaultGasPrice),
			types.NewValueFromUint64(1337),
			[]types.CurrencyBalance{},
			contractAddr,
			execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.waitForReceipt(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)
		blkRef := transport.BlockNumber(receipt.BlockNumber).AsBlockReference()
		_, err = s.tracer.GetBlockTraces(ctx, types.BaseShardId, blkRef)
		s.Require().NoError(err)
	})

	s.Run("ContractDeploy", func() {
		// Deploy counter
		txHash, addr, err := s.rpcClient.DeployContract(s.shardId, s.addrFrom, deployPayload, types.Value{}, execution.MainPrivateKey)
		s.Require().NoError(err)
		s.Require().Equal(contractAddr, addr)

		receipt := s.waitIncludedInMain(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)
	})

	s.Run("Add", func() {
		ctx := context.Background()
		// Add to countuer (state change)
		txHash, err := s.rpcClient.SendMessageViaWallet(
			types.MainWalletAddress,
			contracts.NewCounterAddCallData(s.T(), 5),
			types.Gas(100_000).ToValue(types.DefaultGasPrice),
			types.NewZeroValue(),
			[]types.CurrencyBalance{},
			s.addrFrom,
			execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.waitIncludedInMain(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)

		blkRef := transport.BlockNumber(receipt.BlockNumber).AsBlockReference()
		_, err = s.tracer.GetBlockTraces(ctx, contractAddr.ShardId(), blkRef)
		s.Require().NoError(err)
	})

	s.Run("AllBlocksSerialization", func() {
		testAllBlocksTracesSerialization(s)
	})
}

func (s *TracerTestSuite) TestTestContract() {
	ctx := context.Background()
	deployPayload := contracts.GetDeployPayload(s.T(), contracts.NameTest)
	contractAddr := types.CreateAddress(s.shardId, deployPayload)

	testAddresses := make(map[types.ShardId]types.Address)
	for shardN := range s.nShards {
		shardId := types.ShardId(shardN)
		addr, err := contracts.CalculateAddress(contracts.NameTest, shardId, []byte{byte(shardN)})
		s.Require().NoError(err)
		testAddresses[shardId] = addr
	}

	s.Run("WalletDeploy", func() {
		txHash, err := s.rpcClient.SendMessageViaWallet(
			s.addrFrom,
			types.Code{},
			types.Gas(100_000).ToValue(types.DefaultGasPrice),
			types.NewValueFromUint64(1337),
			[]types.CurrencyBalance{},
			contractAddr,
			execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.waitForReceipt(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)
		blkRef := transport.BlockNumber(receipt.BlockNumber).AsBlockReference()
		_, err = s.tracer.GetBlockTraces(ctx, types.BaseShardId, blkRef)
		s.Require().NoError(err)
	})

	s.Run("ContractDeploy", func() {
		txHash, addr, err := s.rpcClient.DeployContract(s.shardId, s.addrFrom, deployPayload, types.Value{}, execution.MainPrivateKey)
		s.Require().NoError(err)
		s.Require().Equal(contractAddr, addr)

		receipt := s.waitIncludedInMain(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)
	})

	s.Run("EmitEvent", func() {
		ctx := context.Background()
		callData := contracts.NewCallDataT(
			s.T(),
			contracts.NameTest,
			"emitEvent",
			types.NewValueFromUint64(1),
			types.NewValueFromUint64(2),
		)
		txHash, err := s.rpcClient.SendMessageViaWallet(
			types.MainWalletAddress,
			callData,
			types.Gas(100_000).ToValue(types.DefaultGasPrice),
			types.NewZeroValue(),
			[]types.CurrencyBalance{},
			s.addrFrom,
			execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.waitIncludedInMain(txHash)
		s.Require().True(receipt.Success)
		s.Require().Equal("Success", receipt.Status)
		s.Require().Len(receipt.OutReceipts, 1)

		blkRef := transport.BlockNumber(receipt.BlockNumber).AsBlockReference()
		_, err = s.tracer.GetBlockTraces(ctx, contractAddr.ShardId(), blkRef)
		s.Require().NoError(err)
	})

	s.Run("AllBlocksSerialization", func() {
		testAllBlocksTracesSerialization(s)
	})
}

// It looks like even wallet deploy is handled in multiple blocks, I don't know how to catch specific one for
// checks. Just prove every one.
func testAllBlocksTracesSerialization(s *TracerTestSuite) {
	ctx := context.Background()
	for shardN := range s.nShards {
		shardId := types.ShardId(shardN)
		latestBlock, err := s.rpcClient.GetBlock(shardId, "latest", false)
		s.Require().NoError(err)
		for blockNum := range latestBlock.Number {
			blkRef := transport.BlockNumber(blockNum).AsBlockReference()
			s.Require().NoError(err)
			traces, err := s.tracer.GetBlockTraces(ctx, shardId, blkRef)
			s.Require().NoError(err)

			// Test serialization
			tmpfile, err := os.CreateTemp("", "serialized_trace-")
			if err != nil {
				s.Require().NoError(err)
			}
			defer os.Remove(tmpfile.Name())

			err = SerializeToFile(&traces, tmpfile.Name())
			s.Require().NoError(err)
			_, err = DeserializeFromFile(tmpfile.Name())
			s.Require().NoError(err)
		}
	}
}

func TestSuiteTracer(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(TracerTestSuite))
}
