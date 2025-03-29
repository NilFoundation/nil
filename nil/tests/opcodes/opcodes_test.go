package tests

import (
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuitOpcodes struct {
	tests.RpcSuite

	senderAddress1 types.Address

	smartAccountAddress1 types.Address
	smartAccountAddress2 types.Address
}

func (s *SuitOpcodes) SetupSuite() {
	var err error

	s.senderAddress1, err = contracts.CalculateAddress(contracts.NameSender, 1, nil)
	s.Require().NoError(err)

	s.smartAccountAddress1 = contracts.SmartAccountAddress(s.T(), 1, nil, execution.MainPublicKey)
	s.smartAccountAddress2 = contracts.SmartAccountAddress(s.T(), 2, nil, execution.MainPublicKey)

	zeroState := &execution.ZeroStateConfig{
		Contracts: []*execution.ContractDescr{
			{
				Name:     "TestSenderShard1",
				Contract: "tests/Sender",
				Address:  s.senderAddress1,
				Value:    types.NewValueFromUint64(100000000000000),
			},
			{
				Name:     "TestSmartAccountShard1",
				Contract: "SmartAccount",
				Address:  s.smartAccountAddress1,
				Value:    types.Value{},
				CtorArgs: []any{execution.MainPublicKey},
			},
			{
				Name:     "TestSmartAccountShard2",
				Contract: "SmartAccount",
				Address:  s.smartAccountAddress2,
				Value:    types.Value{},
				CtorArgs: []any{execution.MainPublicKey},
			},
		},
	}

	s.Start(&nilservice.Config{
		NShards:   4,
		HttpUrl:   rpc.GetSockPath(s.T()),
		ZeroState: zeroState,
		RunMode:   nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuitOpcodes) TearDownSuite() {
	s.Cancel()
}

func (s *SuitOpcodes) GetBalance(address types.Address) types.Value {
	s.T().Helper()

	balance, err := s.Client.GetBalance(s.Context, address, transport.LatestBlockNumber)
	s.Require().NoError(err)
	return balance
}

func (s *SuitOpcodes) TestSend() {
	// Given
	s.Require().Positive(s.GetBalance(s.senderAddress1).Cmp(types.Value{}))
	s.Require().True(s.GetBalance(s.smartAccountAddress1).IsZero())
	s.Require().True(s.GetBalance(s.smartAccountAddress2).IsZero())

	s.Run("Top up smart account on same shard", func() {
		callData, err := contracts.NewCallData(contracts.NameSender, "send", s.smartAccountAddress1, big.NewInt(100500))
		s.Require().NoError(err)

		txnHash, err := s.Client.SendExternalTransaction(s.Context, callData, s.senderAddress1, nil,
			types.NewFeePackFromGas(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(txnHash)
		s.Require().NotNil(receipt)
		s.Require().True(receipt.Success)

		// Then
		s.Require().Equal(types.NewValueFromUint64(100500), s.GetBalance(s.smartAccountAddress1))
	})

	s.Run("Top up smart account on another shard", func() {
		callData, err := contracts.NewCallData(contracts.NameSender, "send", s.smartAccountAddress2, big.NewInt(100500))
		s.Require().NoError(err)

		txnHash, err := s.Client.SendExternalTransaction(s.Context, callData, s.senderAddress1, nil,
			types.NewFeePackFromGas(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(txnHash)
		s.Require().NotNil(receipt)
		s.Require().False(receipt.Success)

		// Then
		s.Require().True(s.GetBalance(s.smartAccountAddress2).IsZero())
	})
}

func TestSuitOpcodes(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuitOpcodes))
}
