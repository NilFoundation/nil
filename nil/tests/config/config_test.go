package main

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteConfigParams struct {
	tests.RpcSuite
	testAddressMain types.Address
	testAddress     types.Address
	abiTest         *abi.ABI
	abiWallet       *abi.ABI
}

func (s *SuiteConfigParams) SetupSuite() {
	s.ShardsNum = 4

	var err error
	s.testAddressMain, err = contracts.CalculateAddress(contracts.NameConfigTest, types.MainShardId, nil)
	s.Require().NoError(err)

	s.testAddress, err = contracts.CalculateAddress(contracts.NameConfigTest, types.BaseShardId, nil)
	s.Require().NoError(err)

	s.abiWallet, err = contracts.GetAbi("Wallet")
	s.Require().NoError(err)

	s.abiTest, err = contracts.GetAbi(contracts.NameConfigTest)
	s.Require().NoError(err)
}

func (s *SuiteConfigParams) SetupTest() {
	var err error
	s.Db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.Context, s.CtxCancel = context.WithCancel(context.Background())
}

func (s *SuiteConfigParams) TearDownTest() {
	s.Cancel()
}

func (s *SuiteConfigParams) NewValidator() *config.ValidatorInfo {
	s.T().Helper()

	var pubkey [33]byte
	_, err := rand.Read(pubkey[:])
	s.Require().NoError(err)

	address := make([]byte, types.AddrSize)
	_, err = rand.Read(address)
	s.Require().NoError(err)

	return &config.ValidatorInfo{
		PublicKey:         pubkey,
		WithdrawalAddress: types.BytesToAddress(address),
	}
}

func (s *SuiteConfigParams) TestConfigReadWriteValidators() {
	validator1 := s.NewValidator()
	validator2 := s.NewValidator()
	validator3 := s.NewValidator()
	cfg := execution.ZeroStateConfig{
		ConfigParams: execution.ConfigParams{
			Validators: config.ParamValidators{List: []config.ValidatorInfo{*validator1}},
		},
		Contracts: []*execution.ContractDescr{
			{
				Name:     "TestConfig",
				Address:  &s.testAddressMain,
				Value:    types.NewValueFromUint64(10_000_000),
				Contract: contracts.NameConfigTest,
			},
			{
				Name:     "TestConfig",
				Address:  &s.testAddress,
				Value:    types.NewValueFromUint64(10_000_000),
				Contract: contracts.NameConfigTest,
			},
		},
	}
	s.Start(&nilservice.Config{
		NShards:   s.ShardsNum,
		ZeroState: &cfg,
		RunMode:   nilservice.CollatorsOnlyRunMode,
	})

	var (
		receipt *jsonrpc.RPCReceipt
		data    []byte
		vals    config.ParamValidators
	)

	s.Run("Check initial validators", func() {
		vals = config.ParamValidators{List: []config.ValidatorInfo{*validator1}}
		data = s.AbiPack(s.abiTest, "testValidatorsEqual", vals)
		receipt = s.SendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())
	})

	s.Run("Set three new  validators", func() {
		vals = config.ParamValidators{
			List: []config.ValidatorInfo{*validator1, *validator2, *validator3},
		}
		data = s.AbiPack(s.abiTest, "setValidators", vals)
		receipt = s.SendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())

		data = s.AbiPack(s.abiTest, "testValidatorsEqual", vals)
		receipt = s.SendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())

		tx, _ := s.Db.CreateRwTx(s.Context)
		defer tx.Rollback()

		cfgReader, err := config.NewConfigAccessor(tx, types.MainShardId, nil)
		s.Require().NoError(err)
		validators, err := cfgReader.GetParamValidators()
		s.Require().NoError(err)
		s.Require().Equal(vals, *validators)
	})

	s.Run("Check validators from non main shard contract", func() {
		data = s.AbiPack(s.abiTest, "testValidatorsEqual", vals)
		receipt = s.SendExternalMessageNoCheck(data, s.testAddress)
		s.Require().True(receipt.AllSuccess())
	})

	s.Run("Set empty validators", func() {
		vals = config.ParamValidators{List: []config.ValidatorInfo{}}
		data = s.AbiPack(s.abiTest, "setValidators", vals)
		receipt = s.SendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())

		tx, _ := s.Db.CreateRwTx(s.Context)
		defer tx.Rollback()

		cfgReader, err := config.NewConfigAccessor(tx, types.MainShardId, nil)
		s.Require().NoError(err)
		validators, err := cfgReader.GetParamValidators()
		s.Require().NoError(err)
		s.Require().Equal(vals, *validators)
	})
}

func (s *SuiteConfigParams) TestConfigReadWriteGasPrice() {
	cfg := execution.ZeroStateConfig{
		ConfigParams: execution.ConfigParams{
			GasPrice: config.ParamGasPrice{GasPriceScale: *types.NewUint256(10)},
		},
		Contracts: []*execution.ContractDescr{
			{
				Name:     "TestConfig",
				Address:  &s.testAddressMain,
				Value:    types.NewValueFromUint64(10_000_000),
				Contract: contracts.NameConfigTest,
			},
			{
				Name:     "TestConfig",
				Address:  &s.testAddress,
				Value:    types.NewValueFromUint64(10_000_000),
				Contract: contracts.NameConfigTest,
			},
		},
	}

	// Manually set gas price for all shards. It is necessary because the initial prices are set only during the first
	// block generation. But we will likely read config before that.
	cfg.ConfigParams.GasPrice.Shards = make([]types.Uint256, s.ShardsNum-1)
	for i := range s.ShardsNum - 1 {
		cfg.ConfigParams.GasPrice.Shards[i] = *types.DefaultGasPrice.Uint256
	}

	s.Start(&nilservice.Config{
		NShards:              s.ShardsNum,
		Topology:             collate.TrivialShardTopologyId,
		ZeroState:            &cfg,
		CollatorTickPeriodMs: 100,
		RunMode:              nilservice.CollatorsOnlyRunMode,
	})

	var (
		receipt *jsonrpc.RPCReceipt
		data    []byte
	)

	gasPrice := s.readGasPrices()
	gasPrice.GasPriceScale = *types.NewUint256(10)

	s.Run("Check initial gas price param", func() {
		data = s.AbiPack(s.abiTest, "testParamGasPriceEqual", gasPrice)
		receipt = s.SendExternalMessageNoCheck(data, s.testAddress)
		s.Require().True(receipt.AllSuccess())
	})

	s.Run("Modify param", func() {
		gasPrice.GasPriceScale = *types.NewUint256(123)
		data = s.AbiPack(s.abiTest, "setParamGasPrice", gasPrice)
		receipt = s.SendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())

		data = s.AbiPack(s.abiTest, "testParamGasPriceEqual", gasPrice)
		receipt = s.SendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())

		realGasPrice := s.readGasPrices()

		s.Require().Equal(gasPrice.GasPriceScale, realGasPrice.GasPriceScale)
	})

	s.Run("Read param after write", func() {
		data = s.AbiPack(s.abiTest, "readParamAfterWrite")
		receipt = s.SendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())
	})
}

func (s *SuiteConfigParams) readGasPrices() *config.ParamGasPrice {
	s.T().Helper()

	tx, err := s.Db.CreateRoTx(s.Context)
	s.Require().NoError(err)
	defer tx.Rollback()
	cfgReader, err := config.NewConfigAccessorRo(tx, nil)
	s.Require().NoError(err)
	gasPrice, err := cfgReader.GetParamGasPrice()
	s.Require().NoError(err)
	return gasPrice
}

func TestConfig(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteConfigParams))
}
