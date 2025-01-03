package core

import (
	"bytes"
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"
)

const (
	functionSelector = "0x6af78c5c"
)

type ProposerTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	params        *ProposerParams
	db            db.DB
	storage       storage.BlockStorage
	rpcClientMock client.ClientMock
	proposer      *Proposer
}

func TestProposerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ProposerTestSuite))
}

func (s *ProposerTestSuite) SetupSuite() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	logger := logging.NewLogger("proposer_test")
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)

	s.storage = storage.NewBlockStorage(s.db, metricsHandler, logger)
	s.params = NewDefaultProposerParams()
	s.proposer, err = NewProposer(s.ctx, s.params, &s.rpcClientMock, s.storage, metricsHandler, logger)
	s.Require().NoError(err)
}

func (s *ProposerTestSuite) SetupTest() {
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database in SetUpTest")
	s.rpcClientMock.ResetCalls()
}

func (s *ProposerTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *ProposerTestSuite) TestCreateUpdateStateTransaction() {
	oldStateRoot := common.IntToHash(10)
	newStateRoot := common.IntToHash(11)

	transaction, err := s.proposer.createUpdateStateTransaction(oldStateRoot, newStateRoot)
	s.Require().NoError(err, "failed to create transaction")
	s.Require().Equal(s.proposer.seqno.Load(), transaction.Nonce(), "tx nonce is incorrect")

	s.Require().Equal(s.params.ChainId, transaction.ChainId().String(), "tx chainId is incorrect")
	expectedAddress := ethcommon.HexToAddress(s.params.ContractAddress)
	s.Require().Equal(&expectedAddress, transaction.To(), "tx recipient is incorrect")

	functionSelector, err := hexutil.Decode(functionSelector)
	s.Require().NoError(err)
	transactionData := transaction.Data()
	s.Require().True(bytes.Contains(transactionData, functionSelector), "tx data does not contain functionSelector")
	s.Require().True(bytes.Contains(transactionData, oldStateRoot.Bytes()), "tx data does not contain oldStateRoot")
	s.Require().True(bytes.Contains(transactionData, newStateRoot.Bytes()), "tx data does not contain newStateRoot")
}

func (s *ProposerTestSuite) TestSendProof() {
	data := testaide.GenerateProposalData(3)

	err := s.proposer.sendProof(s.ctx, data)
	s.Require().NoError(err, "failed to send proof")

	clientCalls := s.rpcClientMock.RawCallCalls()
	s.Require().Len(clientCalls, 1, "wrong number of calls to rpc client")

	call := clientCalls[0]
	s.Require().Equal("eth_sendRawTransaction", call.Method, "wrong method")
	s.Require().Len(call.Params, 1, "wrong number of passed params")
	s.Require().IsType("", call.Params[0], "wrong type of params[0]")
}
