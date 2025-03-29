package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

type SuiteAccountsBase struct {
	suite.Suite
	db        db.DB
	smcAddr   types.Address
	blockHash common.Hash
}

type SuiteEthAccounts struct {
	SuiteAccountsBase
	api *APIImpl
}

func (suite *SuiteAccountsBase) SetupSuite() {
	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)
}

func (suite *SuiteAccountsBase) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthAccounts) SetupSuite() {
	suite.SuiteAccountsBase.SetupSuite()

	shardId := types.BaseShardId
	ctx := context.Background()

	var err error
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	es, err := execution.NewExecutionState(tx, shardId, execution.StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	suite.Require().NoError(err)
	es.BaseFee = types.DefaultGasPrice

	suite.smcAddr = types.GenerateRandomAddress(shardId)
	suite.Require().NotEmpty(suite.smcAddr)

	suite.Require().NoError(es.CreateAccount(suite.smcAddr))
	suite.Require().NoError(es.SetCode(suite.smcAddr, []byte("some code")))

	suite.Require().NoError(es.SetBalance(suite.smcAddr, types.NewValueFromUint64(1234)))
	suite.Require().NoError(es.SetExtSeqno(suite.smcAddr, 567))

	blockRes, err := es.Commit(0, nil)
	suite.Require().NoError(err)
	suite.blockHash = blockRes.BlockHash

	err = execution.PostprocessBlock(tx, shardId, blockRes, execution.ModeVerify)
	suite.Require().NotNil(blockRes.Block)
	suite.Require().NoError(err)

	err = tx.Commit()
	suite.Require().NoError(err)

	suite.api = NewTestEthAPI(suite.T(), ctx, suite.db, 2)
}

func (suite *SuiteEthAccounts) TearDownSuite() {
	suite.SuiteAccountsBase.TearDownSuite()
}

func (suite *SuiteEthAccounts) TestGetBalance() {
	ctx := context.Background()

	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetBalance(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.NewBigFromInt64(1234), res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetBalance(ctx, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.NewBigFromInt64(1234), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetBalance(ctx, types.GenerateRandomAddress(types.BaseShardId), blockNum)
	suite.Require().NoError(err)
	suite.True(res.IsZero())

	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	res, err = suite.api.GetBalance(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.True(res.IsZero())
}

func (suite *SuiteEthAccounts) TestGetCode() {
	ctx := context.Background()

	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetCode(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes("some code"), res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetCode(ctx, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes("some code"), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetCode(ctx, types.GenerateRandomAddress(types.BaseShardId), blockNum)
	suite.Require().NoError(err)
	suite.Empty(res)

	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	res, err = suite.api.GetCode(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Empty(res)
}

func (suite *SuiteEthAccounts) TestGetSeqno() {
	ctx := context.Background()

	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(ctx, types.GenerateRandomAddress(types.BaseShardId), blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(0), res)

	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	_, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(0), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.PendingBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), res)

	txn := types.ExternalTransaction{
		To:    suite.smcAddr,
		Seqno: 0,
	}

	key, err := crypto.GenerateKey()
	suite.Require().NoError(err)

	digest, err := txn.SigningHash()
	suite.Require().NoError(err)

	txn.AuthData, err = crypto.Sign(digest.Bytes(), key)
	suite.Require().NoError(err)

	data, err := txn.MarshalSSZ()
	suite.Require().NoError(err)

	hash, err := suite.api.SendRawTransaction(ctx, data)
	suite.Require().NoError(err)
	suite.NotEqual(common.EmptyHash, hash)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.PendingBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(1), res)
}

func TestSuiteEthAccounts(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthAccounts))
}
