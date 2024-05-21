package jsonrpc

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/filters"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/stretchr/testify/suite"
)

type SuiteEthFilters struct {
	suite.Suite
	ctx     context.Context
	cancel  context.CancelFunc
	db      db.DB
	api     *APIImpl
	shardId types.ShardId
}

func (s *SuiteEthFilters) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	pool := msgpool.New(msgpool.DefaultConfig)
	s.Require().NotNil(pool)

	s.api = NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), s.db, pool, common.NewLogger("Test", false))
}

func (s *SuiteEthFilters) TearDownTest() {
	s.cancel()
	s.db.Close()
}

func (s *SuiteEthFilters) TestMain() {
	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	address1 := common.HexToAddress("0x1111111111")
	address2 := common.HexToAddress("0x2222222222")

	topics := [][]common.Hash{{}, {}, {{3}}}
	query1 := filters.FilterQuery{
		BlockHash: nil,
		FromBlock: nil,
		ToBlock:   nil,
		Addresses: []common.Address{address1},
		Topics:    topics,
	}
	id1, err := s.api.NewFilter(s.ctx, query1)
	s.Require().NoError(err)
	s.Require().NotEmpty(id1)

	topics2 := [][]common.Hash{{}, {{2}}}
	query2 := filters.FilterQuery{
		BlockHash: nil,
		FromBlock: nil,
		ToBlock:   nil,
		Addresses: []common.Address{},
		Topics:    topics2,
	}
	id2, err := s.api.NewFilter(s.ctx, query2)
	s.Require().NoError(err)
	s.Require().NotEmpty(id2)

	logsInput := []*types.Log{
		{
			Address: address1,
			Topics:  []common.Hash{{0x01}, {0x02}, {0x03}},
			Data:    []byte{0xaa, 0xaa},
		},
		{
			Address: address1,
			Topics:  []common.Hash{{0x03}},
			Data:    []byte{0xbb, 0xbb},
		},
		{
			Address: address1,
			Topics:  []common.Hash{},
			Data:    []byte{0xcc, 0xcc},
		},
		{
			Address: address1,
			Topics:  []common.Hash{{0x03}, {0x04}, {0x03}},
			Data:    []byte{0xaa, 0xaa},
		},
	}
	logsInput2 := []*types.Log{
		{
			Address: address2,
			Topics:  []common.Hash{{0x03}, {0x02}},
			Data:    []byte{0xaa, 0xaa},
		},
	}
	receiptsMpt := mpt.NewMerklePatriciaTrie(s.db, db.ReceiptTrieTableName(s.shardId))

	receipt := &types.Receipt{ContractAddress: address1, Logs: logsInput}
	receiptEncoded, err := receipt.MarshalSSZ()
	s.Require().NoError(err)
	key, err := receipt.HashTreeRoot()
	s.Require().NoError(err)
	s.Require().NoError(receiptsMpt.Set(key[:], receiptEncoded))

	receipt2 := &types.Receipt{ContractAddress: address2, Logs: logsInput2}
	receiptEncoded, err = receipt2.MarshalSSZ()
	s.Require().NoError(err)
	key, err = receipt2.HashTreeRoot()
	s.Require().NoError(err)
	s.Require().NoError(receiptsMpt.Set(key[:], receiptEncoded))

	block := types.Block{
		ReceiptsRoot: receiptsMpt.RootHash(),
	}

	s.Require().NoError(db.WriteBlock(tx, &block))
	s.Require().NoError(tx.Put(db.LastBlockTable, []byte(strconv.Itoa(0)), block.Hash().Bytes()))
	s.Require().NoError(tx.Commit())

	// Wait a bit so the filters detect new block
	time.Sleep(500 * time.Millisecond)

	logs, err := s.api.GetFilterChanges(s.ctx, id1)
	s.Require().NoError(err)
	s.Require().Len(logs, 2)
	s.Require().Equal(logsInput[0], logs[0])
	s.Require().Equal(logsInput[3], logs[1])

	logs, err = s.api.GetFilterChanges(s.ctx, id2)
	s.Require().NoError(err)
	s.Require().Len(logs, 2)
	s.Require().Equal(logsInput[0], logs[0])
	s.Require().Equal(logsInput2[0], logs[1])
}

func TestEthFilters(t *testing.T) {
	suite.Run(t, new(SuiteEthFilters))
}
