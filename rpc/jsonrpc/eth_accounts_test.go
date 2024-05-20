package jsonrpc

import (
	"context"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
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
	suite.Require().NotEqual(common.Address{}, suite.smcAddr)

	err = es.CreateContract(suite.smcAddr, types.Code("some code"))
	suite.Require().NoError(err)

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

	suite.api = NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, pool, common.NewLogger("Test", false))
}

func (suite *SuiteEthAccounts) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthAccounts) TestGetBalance() {
	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetBalance(context.Background(), suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(big.NewInt(1234)), res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetBalance(context.Background(), suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(big.NewInt(1234)), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetBalance(context.Background(), common.HexToAddress("deadbeef"), blockNum)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(big.NewInt(0)), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.EarliestBlock.BlockNumber}
	_, err = suite.api.GetBalance(context.TODO(), suite.smcAddr, blockNum)
	suite.Require().EqualError(err, "not implemented")
}

func (suite *SuiteEthAccounts) TestGetCode() {
	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetCode(context.Background(), suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes("some code"), res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetCode(context.Background(), suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes("some code"), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetCode(context.Background(), common.HexToAddress("deadbeef"), blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes(""), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.EarliestBlock.BlockNumber}
	_, err = suite.api.GetCode(context.TODO(), suite.smcAddr, blockNum)
	suite.Require().EqualError(err, "not implemented")
}

func (suite *SuiteEthAccounts) TestGetSeqno() {
	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetTransactionCount(context.Background(), suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), *res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetTransactionCount(context.Background(), suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), *res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(context.Background(), common.HexToAddress("deadbeef"), blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(0), *res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.EarliestBlock.BlockNumber}
	_, err = suite.api.GetTransactionCount(context.TODO(), suite.smcAddr, blockNum)
	suite.Require().EqualError(err, "not implemented")

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.PendingBlock.BlockNumber}
	_, err = suite.api.GetTransactionCount(context.Background(), suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(0), *res)

	msg := types.Message{
		From:  suite.smcAddr,
		Seqno: 0,
	}
	data, err := msg.MarshalSSZ()
	suite.Require().NoError(err)

	hash, err := suite.api.SendRawTransaction(context.Background(), hexutil.Bytes(data))
	suite.Require().NoError(err)
	suite.NotEqual(common.EmptyHash, hash)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.PendingBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(context.Background(), suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(1), *res)
}

func TestSuiteEthAccounts(t *testing.T) {
	suite.Run(t, new(SuiteEthAccounts))
}
