package jsonrpc

import (
	"context"
	"testing"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
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
	ctx := context.Background()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	mainBlockHash := execution.GenerateZeroState(suite.T(), ctx, types.MainShardId, suite.db)

	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	es, err := execution.NewExecutionState(tx, shardId, mainBlockHash, common.NewTestTimer(0), 1)
	suite.Require().NoError(err)

	suite.smcAddr = types.CreateAddress(shardId, types.BuildDeployPayload([]byte("1234"), common.EmptyHash))
	suite.Require().NotEqual(types.Address{}, suite.smcAddr)

	suite.Require().NoError(es.CreateAccount(suite.smcAddr))

	suite.Require().NoError(es.SetBalance(suite.smcAddr, types.NewValueFromUint64(1234)))
	suite.Require().NoError(es.SetSeqno(suite.smcAddr, 567))

	blockHash, _, err := es.Commit(0)
	suite.Require().NoError(err)
	suite.blockHash = blockHash

	err = tx.Commit()
	suite.Require().NoError(err)

	suite.api = NewTestEthAPI(suite.T(), ctx, suite.db, 1)
}

func (suite *SuiteSendTransaction) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteSendTransaction) TestInvalidMessage() {
	_, err := suite.api.SendRawTransaction(context.Background(), hexutil.Bytes("querty"))
	suite.Require().ErrorIs(err, ssz.ErrSize)
}

func (suite *SuiteSendTransaction) TestInvalidChainId() {
	msg := types.ExternalMessage{
		ChainId: 50,
		To:      types.GenerateRandomAddress(0),
	}

	data, err := msg.MarshalSSZ()
	suite.Require().NoError(err)

	_, err = suite.api.SendRawTransaction(context.Background(), data)
	suite.Require().ErrorContains(err, msgpool.InvalidChainId.String())
}

func (suite *SuiteSendTransaction) TestInvalidShard() {
	msg := types.ExternalMessage{
		To: types.GenerateRandomAddress(1234),
	}

	data, err := msg.MarshalSSZ()
	suite.Require().NoError(err)

	_, err = suite.api.SendRawTransaction(context.Background(), data)
	suite.Require().ErrorContains(err, rawapi.ErrShardNotFound.Error())
}

func TestSuiteSendTransaction(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteSendTransaction))
}
