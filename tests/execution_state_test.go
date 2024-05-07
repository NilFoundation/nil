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

	es, err := execution.NewExecutionState(tx, common.EmptyHash)
	suite.Require().NoError(err)

	addr := common.HexToAddress(
		"9405832983856CB0CF6CD570F071122F1BEA2F20").Hash()

	err = es.CreateContract(addr, []byte("asdf"))
	suite.Require().NoError(err)

	storageKey := common.BytesToHash([]byte("storage-key"))

	err = es.SetState(addr, storageKey, *uint256.NewInt(123456))
	suite.Require().NoError(err)

	blockHash, err := es.Commit()
	suite.Require().NoError(err)

	es, err = execution.NewExecutionState(tx, blockHash)
	suite.Require().NoError(err)

	storageVal, err := es.GetState(addr, storageKey)
	suite.Require().NoError(err)

	suite.Equal(storageVal, *uint256.NewInt(123456))
}

func TestSuiteExecutionState(t *testing.T) {
	suite.Run(t, new(SuiteExecutionState))
}
