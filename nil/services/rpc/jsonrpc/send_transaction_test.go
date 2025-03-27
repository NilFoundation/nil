package jsonrpc

import (
	"context"
	"testing"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/txnpool"
	"github.com/stretchr/testify/suite"
)

type SuiteSendTransaction struct {
	suite.Suite
	db        db.DB
	api       *APIImpl
	smcAddr   types.Address
	blockHash common.Hash
}

func (suite *SuiteSendTransaction) SetupSuite() {
	shardId := types.MainShardId
	ctx := suite.T().Context()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	mainBlock := execution.GenerateZeroState(suite.T(), types.MainShardId, suite.db)

	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	es, err := execution.NewExecutionState(tx, shardId, execution.StateParams{
		Block:          mainBlock,
		ConfigAccessor: config.GetStubAccessor(),
	})
	suite.Require().NoError(err)

	suite.smcAddr = types.CreateAddress(shardId, types.BuildDeployPayload([]byte("1234"), common.EmptyHash))
	suite.Require().NotEqual(types.Address{}, suite.smcAddr)

	suite.Require().NoError(es.CreateAccount(suite.smcAddr))

	suite.Require().NoError(es.SetBalance(suite.smcAddr, types.NewValueFromUint64(1234)))
	suite.Require().NoError(es.SetSeqno(suite.smcAddr, 567))

	blockRes, err := es.Commit(0, nil)
	suite.Require().NoError(err)
	suite.blockHash = blockRes.BlockHash

	err = tx.Commit()
	suite.Require().NoError(err)

	suite.api = NewTestEthAPI(suite.T(), ctx, suite.db, 1)
}

func (suite *SuiteSendTransaction) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteSendTransaction) TestInvalidTransaction() {
	_, err := suite.api.SendRawTransaction(context.Background(), hexutil.Bytes("querty"))
	suite.Require().ErrorIs(err, ssz.ErrSize)
}

func (suite *SuiteSendTransaction) TestInvalidChainId() {
	txn := types.ExternalTransaction{
		ChainId: 50,
		To:      types.GenerateRandomAddress(0),
	}

	data, err := txn.MarshalSSZ()
	suite.Require().NoError(err)

	_, err = suite.api.SendRawTransaction(context.Background(), data)
	suite.Require().ErrorContains(err, txnpool.InvalidChainId.String())
}

func (suite *SuiteSendTransaction) TestInvalidShard() {
	txn := types.ExternalTransaction{
		To: types.GenerateRandomAddress(1234),
	}

	data, err := txn.MarshalSSZ()
	suite.Require().NoError(err)

	_, err = suite.api.SendRawTransaction(context.Background(), data)
	suite.Require().ErrorContains(err, rawapi.ErrShardNotFound.Error())
}

func TestSuiteSendTransaction(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteSendTransaction))
}
