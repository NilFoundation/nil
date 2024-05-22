package jsonrpc

import (
	"context"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
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
	smcAddr   common.Address
	blockHash common.Hash
}

func (suite *SuiteEthAccounts) SetupSuite() {
	shardId := types.MasterShardId
	ctx := context.Background()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)

	es, err := execution.NewExecutionState(tx, shardId, common.EmptyHash)
	suite.Require().NoError(err)

	suite.smcAddr = common.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20")
	suite.Require().NotEmpty(suite.smcAddr)

	es.CreateAccount(suite.smcAddr)
	es.SetCode(suite.smcAddr, []byte("some code"))

	es.SetBalance(suite.smcAddr, *uint256.NewInt(1234))
	es.SetSeqno(suite.smcAddr, 567)

	blockHash, err := es.Commit(0)
	suite.Require().NoError(err)
	suite.blockHash = blockHash

	err = tx.Put(db.LastBlockTable, shardId.Bytes(), blockHash.Bytes())
	suite.Require().NoError(err)

	err = tx.Commit()
	suite.Require().NoError(err)

	pool := msgpool.New(msgpool.DefaultConfig)
	suite.Require().NotNil(pool)

	suite.api = NewEthAPI(ctx, NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, pool, common.NewLogger("Test", false))
}

func (suite *SuiteEthAccounts) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthAccounts) TestGetBalance() {
	ctx := context.Background()
	shardId := types.MasterShardId

	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetBalance(ctx, shardId, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(big.NewInt(1234)), res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetBalance(ctx, shardId, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(big.NewInt(1234)), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	_, err = suite.api.GetBalance(ctx, shardId, common.HexToAddress("deadbeef"), blockNum)
	suite.Require().EqualError(err, "key not found in db")

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.EarliestBlock.BlockNumber}
	_, err = suite.api.GetBalance(ctx, shardId, suite.smcAddr, blockNum)
	suite.Require().EqualError(err, "not implemented")
}

func (suite *SuiteEthAccounts) TestGetCode() {
	ctx := context.Background()
	shardId := types.MasterShardId

	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetCode(ctx, shardId, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes("some code"), res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetCode(ctx, shardId, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes("some code"), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	_, err = suite.api.GetCode(ctx, shardId, common.HexToAddress("deadbeef"), blockNum)
	suite.Require().EqualError(err, "key not found in db")

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.EarliestBlock.BlockNumber}
	_, err = suite.api.GetCode(ctx, shardId, suite.smcAddr, blockNum)
	suite.Require().EqualError(err, "not implemented")
}

func (suite *SuiteEthAccounts) TestGetSeqno() {
	ctx := context.Background()
	shardId := types.MasterShardId

	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetTransactionCount(ctx, shardId, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), *res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetTransactionCount(ctx, shardId, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), *res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	_, err = suite.api.GetTransactionCount(ctx, shardId, common.HexToAddress("deadbeef"), blockNum)
	suite.Require().EqualError(err, "key not found in db")

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.EarliestBlock.BlockNumber}
	_, err = suite.api.GetTransactionCount(ctx, shardId, suite.smcAddr, blockNum)
	suite.Require().EqualError(err, "not implemented")

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.PendingBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(ctx, shardId, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), *res)

	msg := types.Message{
		From:  suite.smcAddr,
		Seqno: 0,
	}

	key, err := crypto.GenerateKey()
	suite.Require().NoError(err)

	digest, err := msg.SigningHash()
	suite.Require().NoError(err)

	sig, err := crypto.Sign(digest.Bytes(), key)
	suite.Require().NoError(err)
	msg.Signature = common.Signature(sig)

	data, err := msg.MarshalSSZ()
	suite.Require().NoError(err)

	hash, err := suite.api.SendRawTransaction(ctx, data)
	suite.Require().NoError(err)
	suite.NotEqual(common.EmptyHash, hash)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.PendingBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(ctx, shardId, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(1), *res)
}

func TestSuiteEthAccounts(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthAccounts))
}
