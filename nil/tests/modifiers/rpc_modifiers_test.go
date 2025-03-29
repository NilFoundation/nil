package tests

import (
	"crypto/ecdsa"
	"testing"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/crypto"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

// This test checks that solidity modifiers `onlyInternal` and `onlyExternal` work correctly.
// To do that it sends internal and external transactions to functions with these modifiers in
// specific contract.

type SuiteModifiersRpc struct {
	tests.RpcSuite
	abi                    *abi.ABI
	smartAccountAddr       types.Address
	smartAccountPrivateKey *ecdsa.PrivateKey
	smartAccountPublicKey  []byte
	testAddr               types.Address
}

func (s *SuiteModifiersRpc) SetupSuite() {
	var err error
	s.smartAccountPrivateKey, s.smartAccountPublicKey, err = crypto.GenerateKeyPair()
	s.Require().NoError(err)

	s.smartAccountAddr = contracts.SmartAccountAddress(s.T(), 2, nil, s.smartAccountPublicKey)
	s.testAddr, err = contracts.CalculateAddress(contracts.NameTransactionCheck, 1, nil)
	s.Require().NoError(err)
	s.abi, err = contracts.GetAbi(contracts.NameTransactionCheck)
	s.Require().NoError(err)

	zeroState := &execution.ZeroStateConfig{
		Contracts: []*execution.ContractDescr{
			{
				Name:     "SmartAccount",
				Contract: "SmartAccount",
				Address:  s.smartAccountAddr,
				Value:    types.NewValueFromUint64(100000000000000000),
				CtorArgs: []interface{}{hexutil.Encode(s.smartAccountPublicKey)},
			},
			{
				Name:     "TransactionCheck",
				Contract: "tests/TransactionCheck",
				Address:  s.testAddr,
				Value:    types.NewValueFromUint64(100000000000000000),
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

func (s *SuiteModifiersRpc) TearDownSuite() {
	s.Cancel()
}

func (s *SuiteModifiersRpc) TestInternalIncorrect() {
	internalFuncCalldata, err := s.abi.Pack("internalFunc")
	s.Require().NoError(err)

	seqno, err := s.Client.GetTransactionCount(s.Context, s.testAddr, "pending")
	s.Require().NoError(err)

	transactionToSend := &types.ExternalTransaction{
		Seqno:        seqno,
		Data:         internalFuncCalldata,
		To:           s.testAddr,
		FeeCredit:    s.GasToValue(100_000),
		MaxFeePerGas: types.MaxFeePerGasDefault,
	}
	s.Require().NoError(transactionToSend.Sign(s.smartAccountPrivateKey))
	txnHash, err := s.Client.SendTransaction(s.Context, transactionToSend)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(txnHash)
	s.Require().False(receipt.Success)
}

func (s *SuiteModifiersRpc) TestInternalCorrect() {
	internalFuncCalldata, err := s.abi.Pack("internalFunc")
	s.Require().NoError(err)

	receipt := s.SendTransactionViaSmartAccount(
		s.smartAccountAddr, s.testAddr, s.smartAccountPrivateKey, internalFuncCalldata)
	s.Require().True(receipt.OutReceipts[0].Success)
}

func (s *SuiteModifiersRpc) TestExternalCorrect() {
	internalFuncCalldata, err := s.abi.Pack("externalFunc")
	s.Require().NoError(err)

	seqno, err := s.Client.GetTransactionCount(s.Context, s.testAddr, "pending")
	s.Require().NoError(err)

	transactionToSend := &types.ExternalTransaction{
		Seqno:        seqno,
		Data:         internalFuncCalldata,
		To:           s.testAddr,
		FeeCredit:    s.GasToValue(100_000),
		MaxFeePerGas: types.MaxFeePerGasDefault,
	}
	s.Require().NoError(transactionToSend.Sign(s.smartAccountPrivateKey))
	txnHash, err := s.Client.SendTransaction(s.Context, transactionToSend)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(txnHash)
	s.Require().True(receipt.Success)
}

func (s *SuiteModifiersRpc) TestExternalIncorrect() {
	internalFuncCalldata, err := s.abi.Pack("externalFunc")
	s.Require().NoError(err)

	receipt := s.SendTransactionViaSmartAccount(
		s.smartAccountAddr, s.testAddr, s.smartAccountPrivateKey, internalFuncCalldata)
	s.Require().False(receipt.OutReceipts[0].Success)
}

func (s *SuiteModifiersRpc) TestExternalSyncCall() {
	internalFuncCalldata, err := s.abi.Pack("callExternal", s.testAddr)
	s.Require().NoError(err)

	seqno, err := s.Client.GetTransactionCount(s.Context, s.testAddr, "pending")
	s.Require().NoError(err)

	transactionToSend := &types.ExternalTransaction{
		Seqno:        seqno,
		Data:         internalFuncCalldata,
		To:           s.testAddr,
		FeeCredit:    s.GasToValue(100_000),
		MaxFeePerGas: types.MaxFeePerGasDefault,
	}
	txnHash, err := s.Client.SendTransaction(s.Context, transactionToSend)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(txnHash)
	s.Require().False(receipt.Success)
}

func (s *SuiteModifiersRpc) TestInternalSyncCall() {
	internalFuncCalldata, err := s.abi.Pack("callInternal", s.testAddr)
	s.Require().NoError(err)

	seqno, err := s.Client.GetTransactionCount(s.Context, s.testAddr, "pending")
	s.Require().NoError(err)

	transactionToSend := &types.ExternalTransaction{
		Seqno:        seqno,
		Data:         internalFuncCalldata,
		To:           s.testAddr,
		FeeCredit:    s.GasToValue(100_000),
		MaxFeePerGas: types.MaxFeePerGasDefault,
	}
	txnHash, err := s.Client.SendTransaction(s.Context, transactionToSend)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(txnHash)
	s.Require().True(receipt.Success)
}

func TestSuiteModifiersRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteModifiersRpc))
}
