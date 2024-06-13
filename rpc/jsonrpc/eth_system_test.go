package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
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

	pool := msgpool.New(msgpool.DefaultConfig)
	suite.Require().NotNil(pool)

	suite.api, err = NewEthAPI(ctx, NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, []msgpool.Pool{pool}, logging.NewLogger("Test"))
	suite.Require().NoError(err)
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
