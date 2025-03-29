package execution

import (
	"context"
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
	"gopkg.in/yaml.v3"
)

type SuiteZeroState struct {
	suite.Suite

	ctx context.Context

	faucetAddr types.Address
	faucetABI  *abi.ABI

	state     *ExecutionState
	contracts map[string]*compiler.Contract
}

func (s *SuiteZeroState) SetupSuite() {
	var err error
	s.ctx = context.Background()

	defaultZeroStateConfig, err := CreateDefaultZeroStateConfig(MainPublicKey)
	s.Require().NoError(err)

	faucetAddress := defaultZeroStateConfig.GetContractAddress("Faucet")
	s.Require().NotNil(faucetAddress)
	s.faucetAddr = faucetAddress

	s.faucetABI, err = contracts.GetAbi(contracts.NameFaucet)
	s.Require().NoError(err)
}

func (s *SuiteZeroState) SetupTest() {
	var err error
	s.state = newState(s.T())

	s.contracts, err = solc.CompileSource("./testdata/call.sol")
	s.Require().NoError(err)
}

func (s *SuiteZeroState) TearDownTest() {
	s.state.tx.Rollback()
}

func (s *SuiteZeroState) getBalance(address types.Address) types.Value {
	s.T().Helper()

	account, ok := s.state.Accounts[address]
	s.Require().True(ok)
	return account.Balance
}

func (s *SuiteZeroState) TestYamlSerialization() {
	orig, err := CreateDefaultZeroStateConfig(MainPublicKey)
	s.Require().NoError(err)

	yamlData, err := yaml.Marshal(orig)
	s.Require().NoError(err)

	deserialized := &ZeroStateConfig{}
	err = yaml.Unmarshal(yamlData, deserialized)
	s.Require().NoError(err)

	s.Require().Equal(orig, deserialized)
}

func (s *SuiteZeroState) TestWithdrawFromFaucet() {
	receiverContract := s.contracts["SimpleContract"]
	receiverAddr := deployContract(s.T(), receiverContract, s.state, 2)
	faucetBalance := s.getBalance(s.faucetAddr)

	calldata, err := s.faucetABI.Pack("withdrawTo", receiverAddr, big.NewInt(100))
	s.Require().NoError(err)

	gasLimit := types.Gas(100_000).ToValue(types.DefaultGasPrice)
	callTransaction := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Data:         calldata,
			To:           s.faucetAddr,
			FeeCredit:    gasLimit,
			MaxFeePerGas: types.MaxFeePerGasDefault,
		},
		From: s.faucetAddr,
	}
	res := s.state.AddAndHandleTransaction(s.ctx, callTransaction, dummyPayer{})
	s.Require().False(res.Failed())

	outTxnHash, ok := reflect.ValueOf(s.state.OutTransactions).MapKeys()[0].Interface().(common.Hash)
	s.Require().True(ok)
	outTxn := s.state.OutTransactions[outTxnHash][0]
	s.Require().NotNil(outTxn)

	res = s.state.AddAndHandleTransaction(s.ctx, outTxn.Transaction, dummyPayer{})
	s.Require().False(res.Failed())

	faucetBalance = faucetBalance.Sub64(100)
	newFaucetBalance := s.getBalance(s.faucetAddr)
	s.Require().Negative(newFaucetBalance.Cmp(faucetBalance))
	s.Require().EqualValues(types.NewValueFromUint64(100), s.getBalance(receiverAddr))
}

func TestZerostateFromConfig(t *testing.T) {
	t.Parallel()

	var state *ExecutionState

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	tx, err := database.CreateRwTx(t.Context())
	require.NoError(t, err)
	defer tx.Rollback()

	configAccessor, err := config.NewConfigAccessorTx(tx, nil)
	require.NoError(t, err)
	state, err = NewExecutionState(tx, 0, StateParams{ConfigAccessor: configAccessor})
	require.NoError(t, err)

	zeroState := &ZeroStateConfig{
		ConfigParams: ConfigParams{
			GasPrice: config.ParamGasPrice{
				Shards: []types.Uint256{*types.NewUint256(1), *types.NewUint256(2), *types.NewUint256(3)},
			},
		},
	}
	err = state.GenerateZeroState(zeroState)
	require.NoError(t, err)
	require.Equal(t, 0, state.GasPrice.Cmp(types.NewValueFromUint64(1)))

	state, err = NewExecutionState(tx, 1, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	require.NoError(t, err)
	zeroState = &ZeroStateConfig{
		ConfigParams: ConfigParams{
			GasPrice: config.ParamGasPrice{
				Shards: []types.Uint256{*types.NewUint256(1), *types.NewUint256(2), *types.NewUint256(3)},
			},
		},
	}

	err = state.GenerateZeroState(zeroState)
	require.NoError(t, err)
	require.Equal(t, 0, state.GasPrice.Cmp(types.NewValueFromUint64(2)))

	state, err = NewExecutionState(tx, 2, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	require.NoError(t, err)
	zeroState = &ZeroStateConfig{
		ConfigParams: ConfigParams{
			GasPrice: config.ParamGasPrice{
				Shards: []types.Uint256{*types.NewUint256(1), *types.NewUint256(2), *types.NewUint256(3)},
			},
		},
	}

	err = state.GenerateZeroState(zeroState)
	require.NoError(t, err)
	require.Equal(t, 0, state.GasPrice.Cmp(types.NewValueFromUint64(3)))

	smartAccountAddr := types.ShardAndHexToAddress(types.MainShardId, "0x111111111111111111111111111111111111")

	state, err = NewExecutionState(tx, types.MainShardId, StateParams{ConfigAccessor: configAccessor})
	require.NoError(t, err)
	zeroState = &ZeroStateConfig{
		Contracts: []*ContractDescr{
			{Name: "Faucet", Value: types.NewValueFromUint64(87654321), Contract: "Faucet"},
			{
				Name:     "MainSmartAccount",
				Contract: "SmartAccount",
				Address:  smartAccountAddr,
				Value:    types.NewValueFromUint64(12345678),
				CtorArgs: []any{MainPublicKey},
			},
		},
	}
	err = state.GenerateZeroState(zeroState)
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
	state, err = NewExecutionState(tx, types.BaseShardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	require.NoError(t, err)
	zeroState = &ZeroStateConfig{
		Contracts: []*ContractDescr{
			{Name: "Faucet"},
		},
	}
	err = state.GenerateZeroState(zeroState)
	require.Error(t, err)

	// Test only one contract should deployed in specific shard
	state, err = NewExecutionState(tx, types.BaseShardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	require.NoError(t, err)
	zeroState = &ZeroStateConfig{
		Contracts: []*ContractDescr{
			{Name: "Faucet", Value: types.NewValueFromUint64(87654321), Contract: "Faucet", Shard: 1},
			{
				Name: "MainSmartAccount", Contract: "SmartAccount", Address: smartAccountAddr,
				Value: types.NewValueFromUint64(12345678), CtorArgs: []any{MainPublicKey},
			},
		},
	}

	err = state.GenerateZeroState(zeroState)
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
