package rpctest

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
	"github.com/stretchr/testify/suite"
)

type SuiteConfigParams struct {
	RpcSuite
	testAddressMain types.Address
	testAddress     types.Address
	abiTest         *abi.ABI
	abiWallet       *abi.ABI
}

func (s *SuiteConfigParams) SetupSuite() {
	s.shardsNum = 4

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
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.context, s.ctxCancel = context.WithCancel(context.Background())
}

func (s *SuiteConfigParams) TearDownTest() {
	s.cancel()
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
	s.start(&nilservice.Config{
		NShards:   s.shardsNum,
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
		receipt = s.sendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())
	})

	s.Run("Set three new  validators", func() {
		vals = config.ParamValidators{
			List: []config.ValidatorInfo{*validator1, *validator2, *validator3},
		}
		data = s.AbiPack(s.abiTest, "setValidators", vals)
		receipt = s.sendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())

		data = s.AbiPack(s.abiTest, "testValidatorsEqual", vals)
		receipt = s.sendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())

		tx, _ := s.db.CreateRwTx(s.context)
		defer tx.Rollback()

		cfgReader, err := config.NewConfigAccessor(tx, types.MainShardId, nil)
		s.Require().NoError(err)
		validators, err := cfgReader.GetParamValidators()
		s.Require().NoError(err)
		s.Require().Equal(vals, *validators)
	})

	s.Run("Check validators from non main shard contract", func() {
		data = s.AbiPack(s.abiTest, "testValidatorsEqual", vals)
		receipt = s.sendExternalMessageNoCheck(data, s.testAddress)
		s.Require().True(receipt.AllSuccess())
	})

	s.Run("Set empty validators", func() {
		vals = config.ParamValidators{List: []config.ValidatorInfo{}}
		data = s.AbiPack(s.abiTest, "setValidators", vals)
		receipt = s.sendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())

		tx, _ := s.db.CreateRwTx(s.context)
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
	s.start(&nilservice.Config{
		NShards:              s.shardsNum,
		Topology:             collate.TrivialShardTopologyId,
		ZeroState:            &cfg,
		CollatorTickPeriodMs: 100,
		RunMode:              nilservice.CollatorsOnlyRunMode,
	})

	var (
		receipt  *jsonrpc.RPCReceipt
		data     []byte
		gasPrice config.ParamGasPrice
	)

	gasPrice.GasPriceScale = *types.NewUint256(10)

	s.Run("Check initial gas price param", func() {
		data = s.AbiPack(s.abiTest, "testParamGasPriceEqual", gasPrice)
		receipt = s.sendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())
	})

	s.Run("Modify param", func() {
		gasPrice.GasPriceScale = *types.NewUint256(123)
		data = s.AbiPack(s.abiTest, "setParamGasPrice", gasPrice)
		receipt = s.sendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())

		data = s.AbiPack(s.abiTest, "testParamGasPriceEqual", gasPrice)
		receipt = s.sendExternalMessageNoCheck(data, s.testAddressMain)
		s.Require().True(receipt.AllSuccess())

		tx, _ := s.db.CreateRwTx(s.context)
		defer tx.Rollback()

		cfgReader, err := config.NewConfigAccessor(tx, types.MainShardId, nil)
		s.Require().NoError(err)
		readGasPrice, err := cfgReader.GetParamGasPrice()
		s.Require().NoError(err)
		s.Require().Equal(gasPrice, *readGasPrice)
	})
}

func TestConfig(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteConfigParams))
}
