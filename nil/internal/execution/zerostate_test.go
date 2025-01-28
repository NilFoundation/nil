package execution

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/tools/solc"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SuiteZeroState struct {
	suite.Suite

	ctx context.Context

	faucetAddr types.Address
	faucetABI  *abi.ABI

	state     *ExecutionState
	contracts map[string]*compiler.Contract
}

func (suite *SuiteZeroState) SetupSuite() {
	suite.ctx = context.Background()

	zeroStateConfig, err := ParseZeroStateConfig(DefaultZeroStateConfig)
	suite.Require().NoError(err)
	faucetAddress := zeroStateConfig.GetContractAddress("Faucet")
	suite.Require().NotNil(faucetAddress)
	suite.faucetAddr = *faucetAddress

	suite.faucetABI, err = contracts.GetAbi(contracts.NameFaucet)
	suite.Require().NoError(err)
}

func (suite *SuiteZeroState) SetupTest() {
	var err error
	suite.state = newState(suite.T())

	suite.contracts, err = solc.CompileSource("./testdata/call.sol")
	suite.Require().NoError(err)
}

func (suite *SuiteZeroState) TearDownTest() {
	suite.state.tx.Rollback()
}

func (suite *SuiteZeroState) getBalance(address types.Address) types.Value {
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

	gasLimit := types.Gas(100_000).ToValue(types.DefaultGasPrice)
	callTransaction := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Data:      calldata,
			To:        suite.faucetAddr,
			FeeCredit: gasLimit,
		},
		From: suite.faucetAddr,
	}
	res := suite.state.HandleTransaction(suite.ctx, callTransaction, dummyPayer{})
	suite.Require().False(res.Failed())

	outTxnHash, ok := reflect.ValueOf(suite.state.OutTransactions).MapKeys()[0].Interface().(common.Hash)
	suite.Require().True(ok)
	outTxn := suite.state.OutTransactions[outTxnHash][0]
	suite.Require().NotNil(outTxn)

	res = suite.state.HandleTransaction(suite.ctx, outTxn.Transaction, dummyPayer{})
	suite.Require().False(res.Failed())

	faucetBalance = faucetBalance.Sub64(100)
	newFaucetBalance := suite.getBalance(suite.faucetAddr)
	suite.Require().Negative(newFaucetBalance.Cmp(faucetBalance))
	suite.Require().EqualValues(types.NewValueFromUint64(100), suite.getBalance(receiverAddr))
}

func TestZerostateFromConfig(t *testing.T) {
	t.Parallel()

	var configYaml string
	var state *ExecutionState

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	tx, err := database.CreateRwTx(context.Background())
	require.NoError(t, err)
	defer tx.Rollback()

	// Test config params
	configYaml = `
config:
  gasPrices: [1, 2, 3]
`
	configAccessor, err := config.NewConfigAccessor(context.Background(), database, nil)
	require.NoError(t, err)
	state, err = NewExecutionState(tx, 0, StateParams{ConfigAccessor: configAccessor})
	require.NoError(t, err)
	err = state.GenerateZeroStateYaml(configYaml)
	require.NoError(t, err)
	require.Equal(t, 0, state.GasPrice.Cmp(types.NewValueFromUint64(1)))

	state, err = NewExecutionState(tx, 1, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	require.NoError(t, err)
	err = state.GenerateZeroStateYaml(configYaml)
	require.NoError(t, err)
	require.Equal(t, 0, state.GasPrice.Cmp(types.NewValueFromUint64(2)))

	state, err = NewExecutionState(tx, 2, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	require.NoError(t, err)
	err = state.GenerateZeroStateYaml(configYaml)
	require.NoError(t, err)
	require.Equal(t, 0, state.GasPrice.Cmp(types.NewValueFromUint64(3)))

	smartAccountAddr := types.ShardAndHexToAddress(types.MainShardId, "0x111111111111111111111111111111111111")

	configYaml = fmt.Sprintf(`
contracts:
- name: Faucet
  value: 87654321
  contract: Faucet
- name: MainSmartAccount
  address: %s
  value: 12345678
  contract: SmartAccount
  ctorArgs: [MainPublicKey]
`, smartAccountAddr.Hex())
	state, err = NewExecutionState(tx, types.MainShardId, StateParams{ConfigAccessor: configAccessor})
	require.NoError(t, err)
	err = state.GenerateZeroStateYaml(configYaml)
	require.NoError(t, err)
	require.Equal(t, types.DefaultGasPrice, state.GasPrice)

	smartAccount, err := state.GetAccount(smartAccountAddr)
	require.NoError(t, err)
	require.NotNil(t, smartAccount)
	require.Equal(t, smartAccount.Balance, types.NewValueFromUint64(12345678))

	faucetCode, err := contracts.GetCode(contracts.NameFaucet)
	require.NoError(t, err)
	faucetAddr := types.CreateAddress(types.MainShardId, types.BuildDeployPayload(faucetCode, common.EmptyHash))

	faucet, err := state.GetAccount(faucetAddr)
	require.NoError(t, err)
	require.NotNil(t, faucet)
	require.Equal(t, faucet.Balance, types.NewValueFromUint64(87654321))

	// Test should fail because contract hasn't `code` item
	configYaml2 := `
contracts:
- name: Faucet
`
	state, err = NewExecutionState(tx, types.BaseShardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	require.NoError(t, err)
	err = state.GenerateZeroStateYaml(configYaml2)
	require.Error(t, err)

	// Test only one contract should deployed in specific shard
	configYaml3 := fmt.Sprintf(`
contracts:
- name: Faucet
  value: 87654321
  shard: 1
  contract: Faucet
- name: MainSmartAccount
  address: %s
  value: 12345678
  contract: SmartAccount
  ctorArgs: [MainPublicKey]
`, smartAccountAddr.Hex())
	state, err = NewExecutionState(tx, types.BaseShardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	require.NoError(t, err)
	err = state.GenerateZeroStateYaml(configYaml3)
	require.NoError(t, err)

	faucetAddr = types.CreateAddress(types.BaseShardId, types.BuildDeployPayload(faucetCode, common.EmptyHash))

	faucet, err = state.GetAccount(faucetAddr)
	require.NoError(t, err)
	require.NotNil(t, faucet)
	smartAccount, err = state.GetAccount(smartAccountAddr)
	require.NoError(t, err)
	require.Nil(t, smartAccount)
}

func TestSuiteZeroState(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteZeroState))
}
