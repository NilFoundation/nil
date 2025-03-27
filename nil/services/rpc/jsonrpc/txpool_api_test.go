package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/txnpool"
	"github.com/stretchr/testify/suite"
)

type SuiteTxnPoolApi struct {
	SuiteAccountsBase
	txnpoolApi *TxPoolAPIImpl
	pool       txnpool.Pool
}

const defaultMaxFee = 500

func newTransaction(address types.Address, seqno types.Seqno, priorityFee uint64, code types.Code) *types.Transaction {
	return &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			To:                   address,
			Seqno:                seqno,
			MaxPriorityFeePerGas: types.NewValueFromUint64(priorityFee),
			MaxFeePerGas:         types.NewValueFromUint64(defaultMaxFee),
			Data:                 code,
		},
	}
}

func (suite *SuiteTxnPoolApi) SetupSuite() {
	suite.SuiteAccountsBase.SetupSuite()
	shardId := types.MainShardId
	var err error
	ctx := context.Background()

	suite.pool, err = txnpool.New(ctx, txnpool.NewConfig(shardId), nil)
	suite.Require().NoError(err)

	database, err := db.NewBadgerDbInMemory()
	suite.Require().NoError(err)
	defer database.Close()

	localShardApis := map[types.ShardId]rawapi.ShardApi{
		shardId: rawapi.NewLocalShardApi(shardId, database, suite.pool),
	}
	localApi := rawapi.NewNodeApiOverShardApis(localShardApis)

	suite.txnpoolApi = NewTxPoolAPI(localApi, logging.NewLogger("Test"))
	suite.Require().NoError(err)
}

func (suite *SuiteTxnPoolApi) TearDownSuite() {
	suite.SuiteAccountsBase.TearDownSuite()
}

func (suite *SuiteTxnPoolApi) TestGetTxnpoolStatus() {
	ctx := context.Background()
	addr1 := types.ShardAndHexToAddress(0, "deadbeef01")
	addr2 := types.ShardAndHexToAddress(0, "deadbeef02")
	txn21 := newTransaction(addr1, 0, 123, types.Code{byte(1)})
	txn22 := newTransaction(addr2, 0, 123, types.Code{byte(2)})
	_, err := suite.pool.Add(ctx, txn21, txn22)
	suite.Require().NoError(err)

	txAmountRes, err := suite.txnpoolApi.GetTxpoolStatus(ctx, types.MainShardId)
	suite.Require().NoError(err)
	suite.Require().Equal(uint64(2), txAmountRes)

	txs, err := suite.txnpoolApi.GetTxpoolContent(ctx, types.MainShardId)
	suite.Require().NoError(err)
	suite.Require().Len(len(txs), 2)
	suite.Require().Equal(txs[0].Data, txn21.Data)
	suite.Require().Equal(txs[1].Data, txn22.Data)
}

func TestSuiteTxnPoolApi(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteTxnPoolApi))
}
