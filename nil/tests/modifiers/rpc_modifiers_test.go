package tests

import (
	"crypto/ecdsa"
	"fmt"
	"testing"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/crypto"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

// This test checks that solidity modifiers `onlyInternal` and `onlyExternal` work correctly.
// To do that it sends internal and external messages to functions with these modifiers in
// specific contract.

type SuiteModifiersRpc struct {
	tests.RpcSuite
	abi              *abi.ABI
	walletAddr       types.Address
	walletPrivateKey *ecdsa.PrivateKey
	walletPublicKey  []byte
	testAddr         types.Address
}

func (s *SuiteModifiersRpc) SetupSuite() {
	var err error
	s.walletPrivateKey, s.walletPublicKey, err = crypto.GenerateKeyPair()
	s.Require().NoError(err)

	s.walletAddr = contracts.WalletAddress(s.T(), 2, nil, s.walletPublicKey)
	s.testAddr, err = contracts.CalculateAddress(contracts.NameMessageCheck, 1, nil)
	s.Require().NoError(err)
	s.abi, err = contracts.GetAbi(contracts.NameMessageCheck)
	s.Require().NoError(err)

	zerostate := fmt.Sprintf(`
contracts:
- name: Wallet
  address: %s
  value: 1000000000000
  contract: Wallet
  ctorArgs: [%s]
- name: MessageCheck
  address: %s
  value: 1000000000000
  contract: tests/MessageCheck
`, s.walletAddr.Hex(), hexutil.Encode(s.walletPublicKey), s.testAddr)

	s.Start(&nilservice.Config{
		NShards:       4,
		HttpUrl:       rpc.GetSockPath(s.T()),
		ZeroStateYaml: zerostate,
		RunMode:       nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuiteModifiersRpc) TearDownSuite() {
	s.Cancel()
}

func (s *SuiteModifiersRpc) TestInternalIncorrect() {
	internalFuncCalldata, err := s.abi.Pack("internalFunc")
	s.Require().NoError(err)

	seqno, err := s.Client.GetTransactionCount(s.testAddr, "pending")
	s.Require().NoError(err)

	messageToSend := &types.ExternalMessage{
		Seqno:     seqno,
		Data:      internalFuncCalldata,
		To:        s.testAddr,
		FeeCredit: s.GasToValue(100_000),
	}
	s.Require().NoError(messageToSend.Sign(s.walletPrivateKey))
	msgHash, err := s.Client.SendMessage(messageToSend)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(s.testAddr.ShardId(), msgHash)
	s.Require().False(receipt.Success)
}

func (s *SuiteModifiersRpc) TestInternalCorrect() {
	internalFuncCalldata, err := s.abi.Pack("internalFunc")
	s.Require().NoError(err)

	receipt := s.SendMessageViaWallet(s.walletAddr, s.testAddr, s.walletPrivateKey, internalFuncCalldata)
	s.Require().True(receipt.OutReceipts[0].Success)
}

func (s *SuiteModifiersRpc) TestExternalCorrect() {
	internalFuncCalldata, err := s.abi.Pack("externalFunc")
	s.Require().NoError(err)

	seqno, err := s.Client.GetTransactionCount(s.testAddr, "pending")
	s.Require().NoError(err)

	messageToSend := &types.ExternalMessage{
		Seqno:     seqno,
		Data:      internalFuncCalldata,
		To:        s.testAddr,
		FeeCredit: s.GasToValue(100_000),
	}
	s.Require().NoError(messageToSend.Sign(s.walletPrivateKey))
	msgHash, err := s.Client.SendMessage(messageToSend)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(s.testAddr.ShardId(), msgHash)
	s.Require().True(receipt.Success)
}

func (s *SuiteModifiersRpc) TestExternalIncorrect() {
	internalFuncCalldata, err := s.abi.Pack("externalFunc")
	s.Require().NoError(err)

	receipt := s.SendMessageViaWallet(s.walletAddr, s.testAddr, s.walletPrivateKey, internalFuncCalldata)
	s.Require().False(receipt.OutReceipts[0].Success)
}

func (s *SuiteModifiersRpc) TestExternalSyncCall() {
	internalFuncCalldata, err := s.abi.Pack("callExternal", s.testAddr)
	s.Require().NoError(err)

	seqno, err := s.Client.GetTransactionCount(s.testAddr, "pending")
	s.Require().NoError(err)

	messageToSend := &types.ExternalMessage{
		Seqno:     seqno,
		Data:      internalFuncCalldata,
		To:        s.testAddr,
		FeeCredit: s.GasToValue(100_000),
	}
	msgHash, err := s.Client.SendMessage(messageToSend)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(s.testAddr.ShardId(), msgHash)
	s.Require().False(receipt.Success)
}

func (s *SuiteModifiersRpc) TestInternalSyncCall() {
	internalFuncCalldata, err := s.abi.Pack("callInternal", s.testAddr)
	s.Require().NoError(err)

	seqno, err := s.Client.GetTransactionCount(s.testAddr, "pending")
	s.Require().NoError(err)

	messageToSend := &types.ExternalMessage{
		Seqno:     seqno,
		Data:      internalFuncCalldata,
		To:        s.testAddr,
		FeeCredit: s.GasToValue(100_000),
	}
	msgHash, err := s.Client.SendMessage(messageToSend)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(s.testAddr.ShardId(), msgHash)
	s.Require().True(receipt.Success)
}

func TestSuiteModifiersRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteModifiersRpc))
}
