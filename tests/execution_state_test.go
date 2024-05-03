package tests

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type SuiteExecutionState struct {
	suite.Suite
	db db.DB
}

func (suite *SuiteExecutionState) SetupTest() {
	var err error
	suite.db, err = db.NewBadgerDb(suite.Suite.T().TempDir() + "test.db")
	suite.Require().NoError(err)
}

func (suite *SuiteExecutionState) TestExecState() {
	tx, err := suite.db.CreateTx(context.TODO())
	suite.Require().NoError(err)

	es, err := execution.NewExecutionState(tx, common.Hash{})
	suite.Require().NoError(err)

	addr := common.HexToAddress(
		"9405832983856CB0CF6CD570F071122F1BEA2F20").Hash()

	err = es.CreateContract(addr, []byte("asdf"))
	suite.Require().NoError(err)

	storage_key := common.BytesToHash([]byte("storage-key"))

	err = es.SetState(addr, storage_key, *uint256.NewInt(123456))
	suite.Require().NoError(err)

	block_hash, err := es.Commit()
	suite.Require().NoError(err)

	es, err = execution.NewExecutionState(tx, block_hash)
	suite.Require().NoError(err)

	storage_val, err := es.GetState(addr, storage_key)
	suite.Require().NoError(err)

	suite.Equal(storage_val, *uint256.NewInt(123456))
}

func TestSuiteExecutionState(t *testing.T) {
	suite.Run(t, new(SuiteExecutionState))
}
