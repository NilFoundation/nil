package jsonrpc

import (
	"context"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type SuiteEthAccounts struct {
	suite.Suite
	db        db.DB
	api       *APIImpl
	smcAddr   types.Address
	blockHash common.Hash
}

func (suite *SuiteEthAccounts) SetupSuite() {
	shardId := types.BaseShardId
	ctx := context.Background()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	es, err := execution.NewExecutionState(tx, shardId, common.EmptyHash, common.NewTestTimer(0))
	suite.Require().NoError(err)

	suite.smcAddr = types.GenerateRandomAddress(shardId)
	suite.Require().NotEmpty(suite.smcAddr)

	suite.Require().NoError(es.CreateAccount(suite.smcAddr))
	suite.Require().NoError(es.SetCode(suite.smcAddr, []byte("some code")))

	suite.Require().NoError(es.SetBalance(suite.smcAddr, *uint256.NewInt(1234)))
	suite.Require().NoError(es.SetSeqno(suite.smcAddr, 567))

	blockHash, err := es.Commit(0)
	suite.Require().NoError(err)
	suite.blockHash = blockHash

	block, err := execution.PostprocessBlock(tx, shardId, blockHash)
	suite.Require().NotNil(block)
	suite.Require().NoError(err)

	err = tx.Commit()
	suite.Require().NoError(err)

	pool := msgpool.New(msgpool.DefaultConfig)
	suite.Require().NotNil(pool)

	suite.api, err = NewEthAPI(ctx,
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, []msgpool.Pool{pool, pool}, logging.NewLogger("Test"))
	suite.Require().NoError(err)
}

func (suite *SuiteEthAccounts) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthAccounts) TestGetBalance() {
	ctx := context.Background()

	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetBalance(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(big.NewInt(1234)), res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetBalance(ctx, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(big.NewInt(1234)), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetBalance(ctx, types.GenerateRandomAddress(types.BaseShardId), blockNum)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(big.NewInt(0)), res)

	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	res, err = suite.api.GetBalance(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(big.NewInt(0)), res)
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
	suite.Nil(res)

	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	res, err = suite.api.GetCode(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Nil(res)
}

func (suite *SuiteEthAccounts) TestGetSeqno() {
	ctx := context.Background()

	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), *res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), *res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(ctx, types.GenerateRandomAddress(types.BaseShardId), blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(0), *res)

	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	_, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(0), *res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.PendingBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), *res)

	msg := types.ExternalMessage{
		To:    suite.smcAddr,
		Seqno: 0,
	}

	key, err := crypto.GenerateKey()
	suite.Require().NoError(err)

	digest, err := msg.SigningHash()
	suite.Require().NoError(err)

	sig, err := crypto.Sign(digest.Bytes(), key)
	suite.Require().NoError(err)
	msg.AuthData = types.Signature(sig)

	data, err := msg.MarshalSSZ()
	suite.Require().NoError(err)

	hash, err := suite.api.SendRawTransaction(ctx, data)
	suite.Require().NoError(err)
	suite.NotEqual(common.EmptyHash, hash)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.PendingBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(1), *res)
}

func TestSuiteEthAccounts(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthAccounts))
}
