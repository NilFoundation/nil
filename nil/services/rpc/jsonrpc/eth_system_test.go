package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/suite"
)

type SuiteEthSystem struct {
	suite.Suite
	db  db.DB
	api *APIImpl
}

func (suite *SuiteEthSystem) SetupSuite() {
	ctx := context.Background()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	err = tx.Commit()
	suite.Require().NoError(err)

	suite.api = NewTestEthAPI(ctx, suite.T(), suite.db, 1)
}

func (suite *SuiteEthSystem) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthSystem) TestChainId() {
	chainId, err := suite.api.ChainId(context.Background())
	suite.Require().NoError(err)
	suite.EqualValues(types.DefaultChainId, chainId)
}

func TestSuiteEthSystem(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthSystem))
}
