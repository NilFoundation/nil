package tests

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/stretchr/testify/suite"
)

type SuiteExecutionState struct {
	suite.Suite
	db db.DB
}

func (suite *SuiteExecutionState) SetupTest() {
	var err error
	suite.db, err = db.NewSqlite(suite.Suite.T().TempDir() + "test.db")
	suite.Require().NoError(err)
}

func (suite *SuiteExecutionState) TestExecState() {
	tx, err := suite.db.CreateTx(context.TODO())
	suite.Require().NoError(err)

	es, err := execution.NewExecutionState(tx, common.Hash{})
	suite.Require().NoError(err)

	addr := common.HexToAddress(
		"9405832983856CB0CF6CD570F071122F1BEA2F20").Hash()
	es.CreateContract(addr, []byte("asdf"))

	es.Commit()
}

func TestSuiteExecutionState(t *testing.T) {
	suite.Run(t, new(SuiteExecutionState))
}
