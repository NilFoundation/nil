package tests

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteConsensus struct {
	tests.ShardedSuite

	testAddress         types.Address
	smartAccountAddress types.Address
	abiTest             *abi.ABI
}

func (s *SuiteConsensus) SetupSuite() {
	var err error
	s.testAddress, err = contracts.CalculateAddress(contracts.NameTest, 1, []byte{1})
	s.Require().NoError(err)
	s.smartAccountAddress = types.MainSmartAccountAddress
	s.abiTest, err = contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)
}

func (s *SuiteConsensus) SetupTest() {
	nShards := uint32(3)

	smartAccountValue, err := types.NewValueFromDecimal("100000000000000000000")
	s.Require().NoError(err)
	zeroState := &execution.ZeroStateConfig{
		Contracts: []*execution.ContractDescr{
			{
				Name:     "MainSmartAccount",
				Contract: "SmartAccount",
				Address:  s.smartAccountAddress,
				Value:    smartAccountValue,
				CtorArgs: []any{execution.MainPublicKey},
			},
			{
				Name:     "Test",
				Contract: "tests/Test",
				Address:  s.testAddress,
				Value:    smartAccountValue,
			},
		},
	}

	s.StartShardAllValidators(&nilservice.Config{
		NShards:              nShards,
		CollatorTickPeriodMs: 200,
		ZeroState:            zeroState,
	}, 10625)
}

func (s *SuiteConsensus) TearDownTest() {
	s.Cancel()
}

func (s *SuiteConsensus) TestConsensus() {
	// Check block id grows
	for _, instance := range s.Instances {
		for _, shardId := range instance.Config.MyShards {
			block, err := instance.Client.GetBlock(s.Context, types.ShardId(shardId), "latest", true)
			s.Require().NoError(err)

			s.Require().Eventually(func() bool {
				newBlock, err := instance.Client.GetBlock(s.Context, types.ShardId(shardId), "latest", true)
				s.Require().NoError(err)
				return newBlock != nil && newBlock.Number > block.Number
			}, 30*time.Second, 1*time.Second)
		}
	}

	// Call smart contract
	data, err := s.abiTest.Pack("saveTime")
	s.Require().NoError(err)
	receipt := tests.SendTransactionViaSmartAccount(
		s.T(), s.Instances[0].Client, s.smartAccountAddress, s.testAddress, execution.MainPrivateKey, data)
	s.Require().True(receipt.AllSuccess())

	// Check that all validators have the same block
	nShards := s.GetNShards()
	blocks := make([]*jsonrpc.RPCBlock, 0, nShards)
	for id := range nShards {
		block, err := s.Instances[0].Client.GetBlock(s.Context, types.ShardId(id), "latest", true)
		s.Require().NoError(err)
		blocks = append(blocks, block)
	}

	for _, block := range blocks {
		for _, instance := range s.Instances {
			instanceBlock := tests.WaitBlock(s.T(), s.Context, instance.Client, block.ShardId, uint64(block.Number))
			s.Equal(block.Hash, instanceBlock.Hash)
		}
	}
}

func TestConsensus(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteConsensus))
}
