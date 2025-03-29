package main

import (
	"context"
	"math/big"
	"sync/atomic"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type TxCountingDB struct {
	db.DB
	roTxCount atomic.Uint64
	rwTxCount atomic.Uint64
}

func (db *TxCountingDB) CreateRoTx(ctx context.Context) (db.RoTx, error) {
	db.roTxCount.Add(1)
	return db.DB.CreateRoTx(ctx)
}

func (db *TxCountingDB) CreateRwTx(ctx context.Context) (db.RwTx, error) {
	db.rwTxCount.Add(1)
	return db.DB.CreateRwTx(ctx)
}

var _ db.DB = new(TxCountingDB)

type SuiteBlockReplay struct {
	tests.RpcSuite
	cfg *nilservice.Config
}

func (s *SuiteBlockReplay) SetupSuite() {
	s.cfg = &nilservice.Config{
		NShards: 4,
		HttpUrl: rpc.GetSockPath(s.T()),
		RunMode: nilservice.NormalRunMode,
	}

	s.DbInit = func() db.DB {
		badger, err := db.NewBadgerDbInMemory()
		s.Require().NoError(err)
		return &TxCountingDB{DB: badger}
	}

	s.Start(s.cfg)
}

func (s *SuiteBlockReplay) restartReplayMode(
	shardId types.ShardId,
	blockNumber types.BlockNumber,
) (roTxCount, rwTxCount uint64) {
	s.T().Helper()

	s.CtxCancel()
	s.Wg.Wait()

	curDb, ok := s.Db.(*TxCountingDB)
	s.Require().True(ok)
	roTxCount = curDb.roTxCount.Load()
	rwTxCount = curDb.rwTxCount.Load()

	s.cfg.RunMode = nilservice.BlockReplayRunMode
	s.cfg.Replay = &nilservice.ReplayConfig{
		ShardId:      shardId,
		BlockIdFirst: blockNumber,
		BlockIdLast:  blockNumber + 1,
	}

	s.DbInit = func() db.DB {
		return s.Db
	}

	s.Start(s.cfg)

	return
}

func (s *SuiteBlockReplay) TearDownTest() {
	s.Cancel()
}

func TestSuiteBlockReplay(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteBlockReplay))
}

func (s *SuiteBlockReplay) TestBlockReplay() {
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/increment.sol"), "Incrementer")
	deployPayload := s.PrepareDefaultDeployPayload(abi, contractCode, big.NewInt(0))

	_, receipt := s.DeployContractViaMainSmartAccount(types.BaseShardId, deployPayload, tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	blockToReplay, err := s.Client.GetBlock(s.Context, types.BaseShardId, receipt.OutReceipts[0].BlockHash, false)
	s.Require().NoError(err)

	// wait 5 next blocks
	var latestBlock *jsonrpc.RPCBlock
	s.Require().Eventually(func() bool {
		block, err := s.Client.GetBlock(s.Context, types.BaseShardId, "latest", false)
		s.Require().NoError(err)
		latestBlock = block
		return block != nil && block.Number > blockToReplay.Number+5
	}, tests.ReceiptWaitTimeout, tests.ReceiptPollInterval)

	s.Run("Replay switches latest block", func() {
		s.restartReplayMode(types.BaseShardId, blockToReplay.Number)

		var newLatestBlock *jsonrpc.RPCBlock

		s.Require().Eventually(func() bool {
			block, err := s.Client.GetBlock(s.Context, types.BaseShardId, "latest", false)
			s.Require().NoError(err)
			newLatestBlock = block
			return block != nil && block.Number < latestBlock.Number && block.Number > blockToReplay.Number
		}, tests.ReceiptWaitTimeout, tests.ReceiptPollInterval)

		s.Require().LessOrEqual(newLatestBlock.Number, blockToReplay.Number+1)
	})

	s.Run("Replay interacts with DB", func() {
		roTxCount, rwTxCount := s.restartReplayMode(types.BaseShardId, blockToReplay.Number)
		db, ok := s.Db.(*TxCountingDB)
		s.Require().True(ok)
		s.Require().Eventually(
			func() bool {
				return db.roTxCount.Load() > roTxCount && db.rwTxCount.Load() > rwTxCount
			},
			tests.ReceiptWaitTimeout,
			tests.ReceiptPollInterval)
	})
}
