package rpctest

import (
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/stretchr/testify/suite"
)

type SuitOpcodes struct {
	RpcSuite

	senderAddress1 types.Address

	walletAddress1 types.Address
	walletAddress2 types.Address
}

func (s *SuitOpcodes) SetupSuite() {
	var err error

	s.senderAddress1, err = contracts.CalculateAddress(contracts.NameSender, 1, nil)
	s.Require().NoError(err)

	s.walletAddress1 = contracts.WalletAddress(s.T(), 1, nil, execution.MainPublicKey)
	s.walletAddress2 = contracts.WalletAddress(s.T(), 2, nil, execution.MainPublicKey)

	zerostateTmpl := `
contracts:
- name: TestSenderShard1
  address: {{ .TestAddress1 }}
  value: 100000000000000
  contract: tests/Sender
  ctorArgs: []
- name: TestWalletShard1
  address: {{ .TestAddress2 }}
  value: 0
  contract: Wallet
  ctorArgs: [{{ .WalletOwnerPublicKey }}]
- name: TestWalletShard2
  address: {{ .TestAddress3 }}
  value: 0
  contract: Wallet
  ctorArgs: [{{ .WalletOwnerPublicKey }}]
`
	zerostate, err := common.ParseTemplate(zerostateTmpl, map[string]interface{}{
		"WalletOwnerPublicKey": hexutil.Encode(execution.MainPublicKey),
		"TestAddress1":         s.senderAddress1.Hex(),
		"TestAddress2":         s.walletAddress1.Hex(),
		"TestAddress3":         s.walletAddress2.Hex(),
	})
	s.Require().NoError(err)

	s.start(&nilservice.Config{
		NShards:              4,
		HttpUrl:              GetSockPath(s.T()),
		Topology:             collate.TrivialShardTopologyId,
		ZeroStateYaml:        zerostate,
		CollatorTickPeriodMs: 100,
		GasPriceScale:        15,
		GasBasePrice:         10,
		RunMode:              nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuitOpcodes) TearDownSuite() {
	s.cancel()
}

func (s *SuitOpcodes) GetBalance(address types.Address) types.Value {
	s.T().Helper()

	balance, err := s.client.GetBalance(address, transport.LatestBlockNumber)
	s.Require().NoError(err)
	return balance
}

func (s *SuitOpcodes) TestSend() {
	// Given
	s.Require().Positive(s.GetBalance(s.senderAddress1).Cmp(types.Value{}))
	s.Require().True(s.GetBalance(s.walletAddress1).IsZero())
	s.Require().True(s.GetBalance(s.walletAddress2).IsZero())

	s.Run("Top up wallet on same shard", func() {
		callData, err := contracts.NewCallData(contracts.NameSender, "send", s.walletAddress1, big.NewInt(100500))
		s.Require().NoError(err)

		msgHash, err := s.client.SendExternalMessage(callData, s.senderAddress1, nil, s.gasToValue(100_000))
		s.Require().NoError(err)
		receipt := s.waitForReceipt(s.senderAddress1.ShardId(), msgHash)
		s.Require().NotNil(receipt)
		s.Require().True(receipt.Success)

		// Then
		s.Require().Equal(types.NewValueFromUint64(100500), s.GetBalance(s.walletAddress1))
	})

	s.Run("Top up wallet on another shard", func() {
		callData, err := contracts.NewCallData(contracts.NameSender, "send", s.walletAddress2, big.NewInt(100500))
		s.Require().NoError(err)

		msgHash, err := s.client.SendExternalMessage(callData, s.senderAddress1, nil, s.gasToValue(100_000))
		s.Require().NoError(err)
		receipt := s.waitForReceipt(s.senderAddress1.ShardId(), msgHash)
		s.Require().NotNil(receipt)
		s.Require().False(receipt.Success)

		// Then
		s.Require().True(s.GetBalance(s.walletAddress2).IsZero())
	})
}

func TestSuitOpcodes(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuitOpcodes))
}
