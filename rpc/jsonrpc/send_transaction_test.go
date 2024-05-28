package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	ssz "github.com/ferranbt/fastssz"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type SuiteSendTransaction struct {
	suite.Suite
	db        db.DB
	api       *APIImpl
	smcAddr   common.Address
	blockHash common.Hash
}

func (suite *SuiteSendTransaction) SetupSuite() {
	shardId := types.MasterShardId
	ctx := context.Background()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)

	es, err := execution.NewExecutionState(tx, shardId, common.EmptyHash, common.NewTestTimer(0))
	suite.Require().NoError(err)

	suite.smcAddr = common.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20")
	suite.Require().NotEqual(common.Address{}, suite.smcAddr)

	es.CreateAccount(suite.smcAddr)

	es.SetBalance(suite.smcAddr, *uint256.NewInt(1234))
	es.SetSeqno(suite.smcAddr, 567)

	blockHash, err := es.Commit(0)
	suite.Require().NoError(err)
	suite.blockHash = blockHash

	err = tx.Commit()
	suite.Require().NoError(err)

	pool := msgpool.New(msgpool.DefaultConfig)
	suite.Require().NotNil(pool)

	suite.api = NewEthAPI(ctx, NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, pool, common.NewLogger("Test"))
}

func (suite *SuiteSendTransaction) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthAccounts) TestInvalidMessage() {
	_, err := suite.api.SendRawTransaction(context.Background(), hexutil.Bytes("querty"))
	suite.Require().ErrorIs(err, ssz.ErrSize)
}

func (suite *SuiteEthAccounts) TestInvalidSignature() {
	msg := types.Message{}

	data, err := msg.MarshalSSZ()
	suite.Require().NoError(err)

	_, err = suite.api.SendRawTransaction(context.Background(), data)
	suite.Require().EqualError(err, "invalid signature")
}

func TestSuiteSendTransaction(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteSendTransaction))
}
