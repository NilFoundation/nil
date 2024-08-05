package db

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/suite"
)

type SuiteVersionInfo struct {
	suite.Suite
	db      DB
	context context.Context
	cancel  context.CancelFunc
}

func (suite *SuiteVersionInfo) SetupTest() {
	var err error
	suite.db, err = NewBadgerDb(suite.Suite.T().TempDir())
	suite.Require().NoError(err)
	suite.context, suite.cancel = context.WithCancel(context.Background())
}

func (suite *SuiteVersionInfo) TearDownTest() {
	suite.db.Close()
	suite.cancel()
}

func (suite *SuiteVersionInfo) TestVersionInfoEmpty() {
	tx, err := suite.db.CreateRoTx(suite.context)
	suite.Require().NoError(err)
	defer tx.Rollback()

	_, err = ReadVersionInfo(tx)
	suite.Require().ErrorIs(err, ErrKeyNotFound)
}

func (suite *SuiteVersionInfo) TestVersionInfoStore() {
	tx, err := suite.db.CreateRwTx(suite.context)
	suite.Require().NoError(err)
	defer tx.Rollback()

	currentVersionInfo := types.NewVersionInfo()
	suite.Require().NoError(WriteVersionInfo(tx, currentVersionInfo))
	suite.Require().NoError(tx.Commit())

	roTx, err := suite.db.CreateRoTx(suite.context)
	suite.Require().NoError(err)
	defer roTx.Rollback()
	dbVersionInfo, err := ReadVersionInfo(roTx)
	suite.Require().NoError(err)
	suite.Require().Equal(dbVersionInfo.Version, currentVersionInfo.Version)
	suite.Require().False(IsVersionOutdated(roTx))
}

func (suite *SuiteVersionInfo) TestVersionInfoOutdated() {
	tx, err := suite.db.CreateRwTx(suite.context)
	suite.Require().NoError(err)
	defer tx.Rollback()

	currentVersionInfo := types.NewVersionInfo()
	outdatedVersionInfo := types.NewVersionInfo()
	outdatedVersionInfo.Version = common.Hash{1} // Make some strange hash to make version outdated
	suite.Require().NoError(WriteVersionInfo(tx, outdatedVersionInfo))
	suite.Require().NoError(tx.Commit())

	roTx, err := suite.db.CreateRoTx(suite.context)
	suite.Require().NoError(err)
	defer roTx.Rollback()
	outdatedVersionInfo, err = ReadVersionInfo(roTx)
	suite.Require().NoError(err)
	suite.Require().NotEqual(outdatedVersionInfo.Version, currentVersionInfo.Version)
	suite.Require().True(IsVersionOutdated(roTx))
}

func TestSuitVersionInfo(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteVersionInfo))
}
