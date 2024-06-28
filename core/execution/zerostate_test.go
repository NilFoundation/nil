package execution

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
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

	state     *ExecutionState
	contracts map[string]*compiler.Contract
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

func (suite *SuiteZeroState) TestWithdrawFromFaucet() {
	receiverContract := suite.contracts["SimpleContract"]
	receiverAddr := deployContract(suite.T(), receiverContract, suite.state, 2)
	faucetBalance := suite.getBalance(suite.faucetAddr)

	calldata, err := suite.faucetABI.Pack("withdrawTo", receiverAddr, big.NewInt(100))
	suite.Require().NoError(err)

	gasLimit := uint64(100_000)
	gasPrice := uint64(10)
	callMessage := &types.Message{
		Data:     calldata,
		From:     suite.faucetAddr,
		To:       suite.faucetAddr,
		GasLimit: *types.NewUint256(gasLimit),
	}
	_, _, err = suite.state.HandleExecutionMessage(suite.ctx, callMessage)
	suite.Require().NoError(err)

	outMsgHash, ok := reflect.ValueOf(suite.state.OutMessages).MapKeys()[0].Interface().(common.Hash)
	suite.Require().True(ok)
	outMsg := suite.state.OutMessages[outMsgHash][0]
	suite.Require().NotNil(outMsg)
	// buy gas
	outMsg.Value.Sub(&outMsg.Value.Int, uint256.NewInt(gasLimit*gasPrice))
	_, _, err = suite.state.HandleExecutionMessage(suite.ctx, outMsg)
	suite.Require().NoError(err)

	faucetBalance.SubUint64(&faucetBalance, 100)
	newFaucetBalance := suite.getBalance(suite.faucetAddr)
	suite.Require().Negative(newFaucetBalance.Cmp(&faucetBalance))
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

	walletAddr := types.ShardAndHexToAddress(types.MasterShardId, "0x111111111111111111111111111111111111")

	configYaml := fmt.Sprintf(`
contracts:
- name: Faucet
  value: 87654321
  contract: Faucet
- name: MainWallet
  address: %s
  value: 12345678
  contract: Wallet
  ctorArgs: [MainPublicKey]
`, walletAddr.Hex())
	err = state.GenerateZeroState(configYaml)
	require.NoError(t, err)

	wallet, err := state.GetAccount(walletAddr)
	require.NoError(t, err)
	require.NotNil(t, wallet)
	require.Equal(t, wallet.Balance, types.NewUint256(12345678).Int)

	faucetCode, err := contracts.GetCode("Faucet")
	require.NoError(t, err)
	faucetAddr := types.CreateAddress(types.MasterShardId, types.BuildDeployPayload(faucetCode, common.EmptyHash))

	faucet, err := state.GetAccount(faucetAddr)
	require.NoError(t, err)
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
	configYaml3 := fmt.Sprintf(`
contracts:
- name: Faucet
  value: 87654321
  shard: 1
  contract: Faucet
- name: MainWallet
  address: %s
  value: 12345678
  contract: Wallet
  ctorArgs: [MainPublicKey]
`, walletAddr.Hex())
	state, err = NewExecutionState(tx, types.BaseShardId, common.EmptyHash, common.NewTestTimer(0))
	require.NoError(t, err)
	err = state.GenerateZeroState(configYaml3)
	require.NoError(t, err)

	faucetAddr = types.CreateAddress(types.BaseShardId, types.BuildDeployPayload(faucetCode, common.EmptyHash))

	faucet, err = state.GetAccount(faucetAddr)
	require.NoError(t, err)
	require.NotNil(t, faucet)
	wallet, err = state.GetAccount(walletAddr)
	require.NoError(t, err)
	require.Nil(t, wallet)
}

func TestSuiteZeroState(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteZeroState))
}
