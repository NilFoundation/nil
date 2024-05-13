package jsonrpc

import (
	"context"
	"math/big"
	"strconv"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type SuiteEthAccounts struct {
	suite.Suite
	db    db.DB
	api   *APIImpl
	smc   types.SmartContract
	block types.Block
}

func (suite *SuiteEthAccounts) SetupSuite() {
	shardId := 0
	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	suite.api = NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, common.NewLogger("Test", false))

	ctx := context.Background()
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	code := types.Code("some code")

	suite.smc = types.SmartContract{
		Address:     common.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20"),
		Initialised: true,
		Balance:     uint256.Int{1234},
		StorageRoot: common.Hash{0x01},
		CodeHash:    code.Hash(),
		Seqno:       567,
	}
	suite.Require().NotEqual(common.Address{}, suite.smc.Address)

	data, err := suite.smc.EncodeSSZ(nil)
	suite.Require().NoError(err)

	root := mpt.NewMerklePatriciaTrie(tx, db.TableName(db.ContractTrieTable, shardId))
	err = root.Set(suite.smc.Address.Hash().Bytes(), data)
	suite.Require().NoError(err)

	suite.block = types.Block{
		Id:                 0,
		PrevBlock:          common.EmptyHash,
		SmartContractsRoot: root.RootHash(),
		MessagesRoot:       common.EmptyHash,
	}
	blockHash := suite.block.Hash()

	err = db.WriteContract(tx, shardId, &suite.smc)
	suite.Require().NoError(err)

	err = db.WriteCode(tx, shardId, code)
	suite.Require().NoError(err)

	err = tx.Put(db.LastBlockTable, []byte(strconv.Itoa(shardId)), blockHash.Bytes())
	suite.Require().NoError(err)

	err = db.WriteBlock(tx, &suite.block)
	suite.Require().NoError(err)

	err = tx.Commit()
	suite.Require().NoError(err)
}

func (suite *SuiteEthAccounts) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthAccounts) TestGetBalance() {
	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetBalance(context.Background(), suite.smc.Address, blockNum)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(suite.smc.Balance.ToBig()), res)

	hash := suite.block.Hash()
	blockHash := transport.BlockNumberOrHash{BlockHash: &hash}
	res, err = suite.api.GetBalance(context.Background(), suite.smc.Address, blockHash)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(suite.smc.Balance.ToBig()), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetBalance(context.Background(), common.HexToAddress("deadbeef"), blockNum)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Big)(big.NewInt(0)), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.EarliestBlock.BlockNumber}
	_, err = suite.api.GetBalance(context.TODO(), suite.smc.Address, blockNum)
	suite.Require().EqualError(err, "not implemented")
}

func (suite *SuiteEthAccounts) TestGetCode() {
	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetCode(context.Background(), suite.smc.Address, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes("some code"), res)

	hash := suite.block.Hash()
	blockHash := transport.BlockNumberOrHash{BlockHash: &hash}
	res, err = suite.api.GetCode(context.Background(), suite.smc.Address, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes("some code"), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetCode(context.Background(), common.HexToAddress("deadbeef"), blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes(""), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.EarliestBlock.BlockNumber}
	_, err = suite.api.GetCode(context.TODO(), suite.smc.Address, blockNum)
	suite.Require().EqualError(err, "not implemented")
}

func (suite *SuiteEthAccounts) TestGetSeqno() {
	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetTransactionCount(context.Background(), suite.smc.Address, blockNum)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Uint64)(&suite.smc.Seqno), res)

	hash := suite.block.Hash()
	blockHash := transport.BlockNumberOrHash{BlockHash: &hash}
	res, err = suite.api.GetTransactionCount(context.Background(), suite.smc.Address, blockHash)
	suite.Require().NoError(err)
	suite.Equal((*hexutil.Uint64)(&suite.smc.Seqno), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(context.Background(), common.HexToAddress("deadbeef"), blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(0), *res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.EarliestBlock.BlockNumber}
	_, err = suite.api.GetTransactionCount(context.TODO(), suite.smc.Address, blockNum)
	suite.Require().EqualError(err, "not implemented")
}

func TestSuiteEthAccounts(t *testing.T) {
	suite.Run(t, new(SuiteEthAccounts))
}
