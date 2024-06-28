package collate

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/suite"
)

type CollatorTestSuite struct {
	suite.Suite

	ctx context.Context
	db  db.DB
}

func (s *CollatorTestSuite) SetupSuite() {
	s.ctx = context.Background()
}

func (s *CollatorTestSuite) SetupTest() {
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
}

func (s *CollatorTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *CollatorTestSuite) TestCollator() {
	shardId := types.BaseShardId
	nShards := 2

	s.Run("GenerateZeroState", func() {
		GenerateZeroState(s.T(), s.ctx, shardId, s.db)
	})

	from := types.MainWalletAddress
	to := contracts.CounterAddress(s.T(), shardId)

	s.Run("SendTokens", func() {
		m := execution.NewExecutionMessage(from, from, 0, contracts.NewWalletSendCallData(s.T(), types.Code{},
			types.NewUint256(100_000), types.NewUint256(3_000_000), []types.CurrencyBalance{},
			to, types.ExecutionMessageKind))
		s.Require().NoError(m.Sign(execution.MainPrivateKey))
		GenerateBlockWithMessages(s.T(), s.ctx, shardId, nShards, s.db, m)
		s.checkReceipt(shardId, m)
	})

	s.Run("ProcessInternalMessage", func() {
		GenerateBlockWithMessages(s.T(), s.ctx, shardId, nShards, s.db)
	})

	s.Run("Deploy", func() {
		m := execution.NewDeployMessage(contracts.CounterDeployPayload(s.T()), shardId, to, 0)
		m.Flags.ClearBit(types.MessageFlagInternal)
		s.Equal(to, m.To)
		GenerateBlockWithMessages(s.T(), s.ctx, shardId, nShards, s.db, m)
		s.checkReceipt(shardId, m)
	})

	s.Run("Execute", func() {
		m := execution.NewExecutionMessage(to, to, 0, contracts.NewCounterAddCallData(s.T(), 3))
		GenerateBlockWithMessages(s.T(), s.ctx, shardId, nShards, s.db, m)
		s.checkReceipt(shardId, m)
	})
}

func (s *CollatorTestSuite) checkReceipt(shardId types.ShardId, m *types.Message) {
	s.T().Helper()

	tx, err := s.db.CreateRoTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	sa, err := execution.NewStateAccessor()
	s.Require().NoError(err)

	msgData, err := sa.Access(tx, m.From.ShardId()).GetInMessage().ByHash(m.Hash())
	s.Require().NoError(err)

	receiptsTrie := execution.NewReceiptTrieReader(mpt.NewReaderWithRoot(tx, shardId, db.ReceiptTrieTable, msgData.Block().ReceiptsRoot))
	receipt, err := receiptsTrie.Fetch(msgData.Index())
	s.Require().NoError(err)
	s.Equal(m.Hash(), receipt.MsgHash)
}

func TestCollator(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CollatorTestSuite{})
}
