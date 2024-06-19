package execution

import (
	"context"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SuiteZeroState struct {
	suite.Suite

	ctx context.Context

	faucetAddr types.Address
	faucetABI  abi.ABI

	state        *ExecutionState
	blockContext *vm.BlockContext
	contracts    map[string]*compiler.Contract
}

func (suite *SuiteZeroState) SetupSuite() {
	suite.ctx = context.Background()

	faucetABI, err := contracts.GetAbi("Faucet")
	suite.Require().NoError(err)
	suite.faucetABI = *faucetABI

	zeroStateConfig, err := ParseZeroStateConfig(DefaultZeroStateConfig)
	suite.Require().NoError(err)
	faucetAddress := zeroStateConfig.GetContractAddress("Faucet")
	suite.Require().NotNil(faucetAddress)
	suite.faucetAddr = *faucetAddress
}

func (suite *SuiteZeroState) SetupTest() {
	var err error
	suite.state = newState(suite.T())

	suite.blockContext, err = NewEVMBlockContext(suite.state)
	suite.Require().NoError(err)

	suite.contracts, err = solc.CompileSource("./testdata/call.sol")
	suite.Require().NoError(err)
}

func (suite *SuiteZeroState) TearDownTest() {
}

func (suite *SuiteZeroState) getBalance(address types.Address) uint256.Int {
	suite.T().Helper()

	account, ok := suite.state.Accounts[address]
	suite.Require().True(ok)
	return account.Balance
}

func (suite *SuiteZeroState) TestFaucetBalance() {
	ret := suite.getBalance(suite.faucetAddr)
	suite.Require().EqualValues(*uint256.NewInt(1000000000000), ret)
}

func (suite *SuiteZeroState) TestWithdrawFromFaucet() {
	receiverContract := suite.contracts["SimpleContract"]
	receiverAddr := deployContract(suite.T(), receiverContract, suite.state, suite.blockContext, 2)

	calldata, err := suite.faucetABI.Pack("withdrawTo", receiverAddr, big.NewInt(100))
	suite.Require().NoError(err)

	callMessage := &types.Message{
		Data:     calldata,
		From:     suite.faucetAddr,
		To:       suite.faucetAddr,
		GasLimit: *types.NewUint256(10000),
	}
	_, _, err = suite.state.HandleExecutionMessage(suite.ctx, callMessage, suite.blockContext)
	suite.Require().NoError(err)

	suite.Require().EqualValues(*uint256.NewInt(1000000000000 - 100), suite.getBalance(suite.faucetAddr))
	suite.Require().EqualValues(*uint256.NewInt(100), suite.getBalance(receiverAddr))
}

func TestZerostateFromConfig(t *testing.T) {
	t.Parallel()

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	tx, err := database.CreateRwTx(context.Background())
	require.NoError(t, err)
	state, err := NewExecutionState(tx, types.MasterShardId, common.EmptyHash, common.NewTestTimer(0))
	require.NoError(t, err)

	configYaml := `
contracts:
- name: Faucet
  value: 87654321
  contract: Faucet
- name: MainWallet
  address: 0x0000111111111111111111111111111111111111
  value: 12345678
  contract: Wallet
  ctorArgs: [MainPublicKey]
`
	err = state.GenerateZeroState(configYaml)
	require.NoError(t, err)

	wallet := state.GetAccount(types.MainWalletAddress)
	require.NotNil(t, wallet)
	require.Equal(t, wallet.Balance, types.NewUint256(12345678).Int)

	faucetCode, err := contracts.GetCode("Faucet")
	require.NoError(t, err)
	faucetAddr := types.CreateAddress(types.MasterShardId, faucetCode)

	faucet := state.GetAccount(faucetAddr)
	require.NotNil(t, faucet)
	require.Equal(t, faucet.Balance, types.NewUint256(87654321).Int)

	// Test should fail because contract hasn't `code` item
	configYaml2 := `
contracts:
- name: Faucet
`
	state, err = NewExecutionState(tx, types.BaseShardId, common.EmptyHash, common.NewTestTimer(0))
	require.NoError(t, err)
	err = state.GenerateZeroState(configYaml2)
	require.Error(t, err)

	// Test only one contract should deployed in specific shard
	configYaml3 := `
contracts:
- name: Faucet
  value: 87654321
  shard: 1
  contract: Faucet
- name: MainWallet
  address: 0x0000111111111111111111111111111111111111
  value: 12345678
  contract: Wallet
  ctorArgs: [MainPublicKey]
`
	state, err = NewExecutionState(tx, types.BaseShardId, common.EmptyHash, common.NewTestTimer(0))
	require.NoError(t, err)
	err = state.GenerateZeroState(configYaml3)
	require.NoError(t, err)

	faucetAddr = types.CreateAddress(types.BaseShardId, faucetCode)

	faucet = state.GetAccount(faucetAddr)
	require.NotNil(t, faucet)
	wallet = state.GetAccount(types.MainWalletAddress)
	require.Nil(t, wallet)
}

func TestSuiteZeroState(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteZeroState))
}
