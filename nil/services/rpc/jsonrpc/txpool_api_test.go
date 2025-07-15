package jsonrpc

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

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
	api        rawapi.NodeApi
	pool       txnpool.Pool
}

const defaultMaxFee = 500

func newTransaction(address types.Address, seqno types.Seqno, priorityFee uint64, code types.Code) *types.Transaction {
	extTxn := &types.ExternalTransaction{
		To:    address,
		Seqno: seqno,
		Data:  code,
		FeePack: types.FeePack{
			MaxPriorityFeePerGas: types.NewValueFromUint64(priorityFee),
			MaxFeePerGas:         types.NewValueFromUint64(defaultMaxFee),
		},
	}
	// extTxn := &types.ExternalTransaction{
	// 	To:      contractAddress,
	// 	Data:    bytecode,
	// 	Seqno:   seqno,
	// 	Kind:    kind,
	// 	FeePack: fee,
	// }
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		panic(err) // TODO
	}
	extTxn.Sign(privateKey)
	txn, err := extTxn.ToTransaction()
	if err != nil {
		panic(err)
	}
	return txn
}

func (suite *SuiteTxnPoolApi) SetupSuite() {
	suite.SuiteAccountsBase.SetupSuite()
	var err error

	suite.pool, err = txnpool.New(suite.T().Context(), txnpool.NewConfig(types.MainShardId), nil)
	suite.Require().NoError(err)

	database, err := db.NewBadgerDbInMemory()
	suite.Require().NoError(err)
	defer database.Close()

	suite.api = rawapi.NodeApiBuilder(database, nil).
		WithLocalShardApiRw(types.MainShardId, suite.pool).
		BuildAndReset()
	suite.txnpoolApi = NewTxPoolAPI(suite.api, logging.NewLogger("Test"))
	suite.Require().NoError(err)
}

func (suite *SuiteTxnPoolApi) TearDownSuite() {
	suite.SuiteAccountsBase.TearDownSuite()
}

func (suite *SuiteTxnPoolApi) TestTnxApi() {
	ctx := context.Background()
	transactionAmount := uint64(10)

	for i := range transactionAmount {
		addr := types.ShardAndHexToAddress(0, "deadbeef"+fmt.Sprintf("%02d", i))
		txn := newTransaction(addr, 0, 123, types.Code{byte(i)})
		_, err := suite.pool.Add(ctx, txn)
		suite.Require().NoError(err)
	}

	suite.Run("NodeApi", func() {
		txAmountRes, err := suite.api.GetTxpoolStatus(ctx, types.MainShardId)
		suite.Require().NoError(err)
		suite.Require().Equal(transactionAmount, txAmountRes)

		txs, err := suite.api.GetTxpoolContent(ctx, types.MainShardId)
		suite.Require().NoError(err)
		txsContentAmount := uint64(len(txs))
		suite.Require().Equal(transactionAmount, txsContentAmount)
	})

	suite.Run("TxnpoolApi", func() {
		txAmountRes, err := suite.txnpoolApi.GetTxpoolStatus(ctx, types.MainShardId)
		suite.Require().NoError(err)
		suite.Require().Equal(transactionAmount, txAmountRes.Pending)

		txs, err := suite.txnpoolApi.GetTxpoolContent(ctx, types.MainShardId)
		suite.Require().NoError(err)
		txsContentAmount := uint64(len(txs.Pending))
		suite.Require().Equal(transactionAmount, txsContentAmount)
	})
}

func TestSuiteTxnPoolApi(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteTxnPoolApi))
}
