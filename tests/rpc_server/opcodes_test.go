package rpctest

import (
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/holiman/uint256"
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

	s.senderAddress1, err = contracts.CalculateAddress("tests/Sender", 1, []any{}, []byte{0})
	s.Require().NoError(err)

	s.walletAddress1, err = contracts.CalculateAddress("Wallet", 1, []any{execution.MainPublicKey}, []byte{0})
	s.Require().NoError(err)

	s.walletAddress2, err = contracts.CalculateAddress("Wallet", 2, []any{execution.MainPublicKey}, []byte{0})
	s.Require().NoError(err)

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
		HttpPort:             8536,
		Topology:             collate.TrivialShardTopologyId,
		ZeroState:            zerostate,
		CollatorTickPeriodMs: 100,
		GracefulShutdown:     false,
		GasPriceScale:        15,
		GasBasePrice:         10,
	})
}

func (s *SuitOpcodes) GetBalance(address types.Address) *types.Uint256 {
	s.T().Helper()

	balance, err := s.client.GetBalance(address, transport.LatestBlockNumber)
	s.Require().NoError(err)
	return balance
}

func (s *SuitOpcodes) CmpBalance(address types.Address, value uint64) int {
	s.T().Helper()

	return s.GetBalance(address).Cmp(uint256.NewInt(value))
}

func (s *SuitOpcodes) TestSend() {
	// Given
	s.Require().Positive(s.GetBalance(s.senderAddress1).Cmp(uint256.NewInt(0)))
	s.Require().Zero(s.GetBalance(s.walletAddress1).Cmp(uint256.NewInt(0)))
	s.Require().Zero(s.GetBalance(s.walletAddress2).Cmp(uint256.NewInt(0)))

	s.Run("Top up wallet on same shard", func() {
		// When
		senderAbi, err := contracts.GetAbi("tests/Sender")
		s.Require().NoError(err)
		calldata, err := senderAbi.Pack("send", s.walletAddress1, big.NewInt(100500))
		s.Require().NoError(err)

		msgHash, err := s.client.SendExternalMessage(calldata, s.senderAddress1, nil)
		s.Require().NoError(err)
		receipt := s.waitForReceipt(s.senderAddress1.ShardId(), msgHash)
		s.Require().NotNil(receipt)
		s.Require().True(receipt.Success)

		// Then
		s.Require().Equal(types.NewUint256(100500), s.GetBalance(s.walletAddress1))
	})

	s.Run("Top up wallet on another shard", func() {
		// When
		senderAbi, err := contracts.GetAbi("tests/Sender")
		s.Require().NoError(err)
		calldata, err := senderAbi.Pack("send", s.walletAddress2, big.NewInt(100500))
		s.Require().NoError(err)

		msgHash, err := s.client.SendExternalMessage(calldata, s.senderAddress1, nil)
		s.Require().NoError(err)
		receipt := s.waitForReceipt(s.senderAddress1.ShardId(), msgHash)
		s.Require().NotNil(receipt)
		s.Require().False(receipt.Success)

		// Then
		s.Require().Zero(s.GetBalance(s.walletAddress2).Cmp(uint256.NewInt(0)))
	})
}

func TestSuitOpcodes(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuitOpcodes))
}
