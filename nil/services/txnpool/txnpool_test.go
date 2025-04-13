package txnpool

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type SuiteTxnPool struct {
	suite.Suite

	ctx    context.Context
	cancel context.CancelFunc

	pool *TxnPool
}

var defaultAddress = types.ShardAndHexToAddress(0, "11")

const defaultMaxFee = 500

var defaultBaseFee = types.NewValueFromUint64(100)

func newTransaction(address types.Address, seqno types.Seqno, priorityFee uint64) *types.Transaction {
	return &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			To:                   address,
			Seqno:                seqno,
			MaxPriorityFeePerGas: types.NewValueFromUint64(priorityFee),
			MaxFeePerGas:         types.NewValueFromUint64(defaultMaxFee),
		},
	}
}

func newTransaction2(address types.Address, seqno types.Seqno, priorityFee, maxFee, value uint64) *types.Transaction {
	return &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			To:                   address,
			Seqno:                seqno,
			MaxPriorityFeePerGas: types.NewValueFromUint64(priorityFee),
			MaxFeePerGas:         types.NewValueFromUint64(maxFee),
		},
		Value: types.NewValueFromUint64(value),
	}
}

func (s *SuiteTxnPool) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	var err error
	s.pool, err = New(s.ctx, NewConfig(0), nil)
	s.Require().NoError(err)
}

func (s *SuiteTxnPool) TearDownTest() {
	s.cancel()
}

func (s *SuiteTxnPool) addTransactionsToPoolSuccessfully(pool Pool, txn ...*types.Transaction) {
	s.T().Helper()

	count := pool.GetSize()

	reasons, err := pool.Add(s.ctx, txn...)
	s.Require().NoError(err)
	s.Require().Len(reasons, len(txn))
	for _, reason := range reasons {
		s.Equal(NotSet, reason)
	}

	s.Equal(count+len(txn), pool.GetSize())
}

func (s *SuiteTxnPool) addTransactions(txn ...*types.Transaction) []DiscardReason {
	s.T().Helper()

	reasons, err := s.pool.Add(s.ctx, txn...)
	s.Require().NoError(err)
	s.Require().Len(reasons, len(txn))

	return reasons
}

func (s *SuiteTxnPool) getTransactions() []*types.TxnWithHash {
	s.T().Helper()

	res, err := s.pool.Peek(100000)
	s.Require().NoError(err)
	return res
}

func (s *SuiteTxnPool) addTransactionsSuccessfully(txn ...*types.Transaction) {
	s.T().Helper()

	s.addTransactionsToPoolSuccessfully(s.pool, txn...)
}

func (s *SuiteTxnPool) addTransactionWithDiscardReason(txn *types.Transaction, reason DiscardReason) {
	s.T().Helper()

	count := s.getTransactionCount(s.pool)

	reasons, err := s.pool.Add(s.ctx, txn)
	s.Require().NoError(err)
	s.Equal([]DiscardReason{reason}, reasons)

	s.Equal(count, s.getTransactionCount(s.pool))
}

func (s *SuiteTxnPool) TestAdd() {
	wrongShardTxn := newTransaction(defaultAddress, 0, 123)
	wrongShardTxn.To = types.ShardAndHexToAddress(1, "deadbeef")
	_, err := s.pool.Add(s.ctx, wrongShardTxn)
	s.Require().Error(err)

	txn1 := newTransaction(defaultAddress, 0, 123)

	// Send the transaction for the first time - OK
	s.addTransactionsSuccessfully(txn1)

	// Send transaction once again - Duplicate hash
	s.addTransactionWithDiscardReason(txn1, DuplicateHash)

	// Send the same transaction with higher fee - OK
	// Doesn't use the same helper because here transaction count doesn't change
	txn2 := common.CopyPtr(txn1)
	txn2.MaxPriorityFeePerGas = txn2.MaxPriorityFeePerGas.Add64(100)
	reasons, err := s.pool.Add(s.ctx, txn2)
	s.Require().NoError(err)
	s.Require().Equal([]DiscardReason{NotSet}, reasons)
	s.Equal(1, s.getTransactionCount(s.pool))

	// Add a transaction with higher seqno to the same receiver
	tx := newTransaction(defaultAddress, 1, 124)
	s.addTransactionsSuccessfully(tx)

	err = s.pool.OnCommitted(s.ctx, defaultBaseFee, []*types.Transaction{tx})
	s.Require().NoError(err)

	// Add a transaction with lower seqno to the same receiver - SeqnoTooLow
	s.addTransactionWithDiscardReason(
		newTransaction(defaultAddress, 0, 124), SeqnoTooLow)

	// Add a transaction with higher seqno to a new receiver
	otherAddressTxn := newTransaction(types.ShardAndHexToAddress(0, "deadbeef01"), 1, 124)
	s.addTransactionsSuccessfully(otherAddressTxn)
}

func (s *SuiteTxnPool) TestAddOverflow() {
	s.pool.cfg.Size = 1

	s.addTransactionsSuccessfully(
		newTransaction(defaultAddress, 0, 123))

	s.addTransactionWithDiscardReason(
		newTransaction(defaultAddress, 1, 123), PoolOverflow)
}

func (s *SuiteTxnPool) TestStarted() {
	s.True(s.pool.Started())
}

func (s *SuiteTxnPool) TestIdHashKnownGet() {
	txn := newTransaction(defaultAddress, 0, 123)
	s.addTransactionsSuccessfully(txn)

	has, err := s.pool.IdHashKnown(txn.Hash())
	s.Require().NoError(err)
	s.True(has)

	poolTxn, err := s.pool.Get(txn.Hash())
	s.Require().NoError(err)
	s.Equal(poolTxn, txn)

	has, err = s.pool.IdHashKnown(common.BytesToHash([]byte("abcd")))
	s.Require().NoError(err)
	s.False(has)

	poolTxn, err = s.pool.Get(common.BytesToHash([]byte("abcd")))
	s.Require().NoError(err)
	s.Nil(poolTxn)
}

func (s *SuiteTxnPool) TestSeqnoFromAddress() {
	txn := newTransaction(defaultAddress, 0, 123)

	_, inPool := s.pool.SeqnoToAddress(txn.To)
	s.Require().False(inPool)

	s.addTransactionsSuccessfully(txn)

	seqno, inPool := s.pool.SeqnoToAddress(txn.To)
	s.Require().True(inPool)
	s.Require().EqualValues(0, seqno)

	nextTxn := common.CopyPtr(txn)
	nextTxn.Seqno++
	s.addTransactionsSuccessfully(nextTxn)

	seqno, inPool = s.pool.SeqnoToAddress(txn.To)
	s.Require().True(inPool)
	s.Require().EqualValues(1, seqno)

	_, inPool = s.pool.SeqnoToAddress(types.BytesToAddress([]byte("abcd")))
	s.Require().False(inPool)
}

func (s *SuiteTxnPool) TestSeqnoGap() {
	txn0 := newTransaction(defaultAddress, 0, 123)
	txn1 := newTransaction(defaultAddress, 1, 123)
	txn2 := newTransaction(defaultAddress, 2, 123)
	txn3 := newTransaction(defaultAddress, 3, 123)
	s.addTransactionsSuccessfully(
		txn0,
		txn3)

	txns, err := s.pool.Peek(0)
	s.Require().NoError(err)
	s.Len(txns, 1)
	s.Require().Equal(txn0, txns[0].Transaction)

	err = s.pool.OnCommitted(s.ctx, defaultBaseFee, []*types.Transaction{txn0})
	s.Require().NoError(err)

	s.addTransactionsSuccessfully(
		txn2,
		txn1)

	txns, err = s.pool.Peek(0)
	s.Require().NoError(err)
	s.Len(txns, 3)
	s.Require().Equal(txn1, txns[0].Transaction)
	s.Require().Equal(txn2, txns[1].Transaction)
	s.Require().Equal(txn3, txns[2].Transaction)
}

func (s *SuiteTxnPool) TestPeek() {
	address2 := types.ShardAndHexToAddress(0, "deadbeef02")

	txn21 := newTransaction(address2, 0, 123)
	txn22 := newTransaction(address2, 1, 123)

	s.addTransactionsSuccessfully(
		newTransaction(defaultAddress, 0, 123),
		newTransaction(defaultAddress, 1, 123),
		txn21,
		txn22)

	txns, err := s.pool.Peek(1)
	s.Require().NoError(err)
	s.Len(txns, 1)

	txns, err = s.pool.Peek(4)
	s.Require().NoError(err)
	s.Len(txns, 4)

	txns, err = s.pool.Peek(10)
	s.Require().NoError(err)
	s.Len(txns, 4)
}

func (s *SuiteTxnPool) TestOnNewBlock() {
	address2 := types.ShardAndHexToAddress(0, "deadbeef02")

	txn11 := newTransaction(defaultAddress, 0, 123)
	txn12 := newTransaction(defaultAddress, 1, 123)

	txn21 := newTransaction(address2, 0, 123)
	txn22 := newTransaction(address2, 1, 123)

	s.addTransactionsSuccessfully(txn11, txn12, txn21, txn22)

	// TODO: Ideally we need to do that via execution state
	err := s.pool.OnCommitted(s.ctx, defaultBaseFee, []*types.Transaction{txn11, txn12, txn21})
	s.Require().NoError(err)

	// After commit Peek should return only one transaction
	transactions, err := s.pool.Peek(10)
	s.Require().NoError(err)
	s.Require().Len(transactions, 1)
	s.Equal(types.NewTxnWithHash(txn22), transactions[0])
	s.Equal(1, s.getTransactionCount(s.pool))
}

func (s *SuiteTxnPool) TestBaseFeeChanged() {
	address2 := types.ShardAndHexToAddress(0, "22")

	err := s.pool.OnCommitted(s.ctx, types.NewValueFromUint64(100), nil)
	s.Require().NoError(err)

	txn11 := newTransaction2(defaultAddress, 0, 5, 110, 0)  // 5
	txn12 := newTransaction2(defaultAddress, 1, 50, 125, 1) // 25

	txn21 := newTransaction2(address2, 0, 20, 95, 2)  // -5
	txn22 := newTransaction2(address2, 1, 30, 135, 3) // 30

	s.addTransactions(txn11, txn12, txn21, txn22)
	s.checkTransactionsOrder(3, 0, 1)

	err = s.pool.OnCommitted(s.ctx, types.NewValueFromUint64(120), nil)
	s.Require().NoError(err)
	// Now:
	// txn11: -10
	// txn12: 5
	// txn21: -25
	// txn22: 15
	s.checkTransactionsOrder(3, 1)

	// Transactions with smaller seqno(txn11, txn21) should be removed either
	err = s.pool.OnCommitted(s.ctx, types.NewValueFromUint64(80), []*types.Transaction{txn12, txn22})
	s.Require().NoError(err)
	s.checkTransactionsOrder()
}

func (s *SuiteTxnPool) TestReplacement() {
	err := s.pool.OnCommitted(s.ctx, types.NewValueFromUint64(1000), nil)
	s.Require().NoError(err)

	txn1 := newTransaction2(defaultAddress, 0, 100, 1100, 0)
	s.addTransactions(txn1)
	s.checkTransactionsOrder(0)

	// Not replaced: new priorityFee is less than FeeBumpPercentage
	txn1 = newTransaction2(defaultAddress, 0, 100+FeeBumpPercentage-1, 1200, 1)
	reasons := s.addTransactions(txn1)
	s.checkTransactionsOrder(0)
	s.Require().Equal([]DiscardReason{NotReplaced}, reasons)

	// Replaced: new priorityFee is equal to FeeBumpPercentage
	txn1 = newTransaction2(defaultAddress, 0, 100+FeeBumpPercentage, 1200, 2)
	s.addTransactions(txn1)
	s.checkTransactionsOrder(2)

	// Not replaced: maxFeePerGas is small
	txn1 = newTransaction2(defaultAddress, 0, 150, 1100, 3)
	reasons = s.addTransactions(txn1)
	s.checkTransactionsOrder(2)
	s.Require().Equal([]DiscardReason{NotReplaced}, reasons)
}

func (s *SuiteTxnPool) TestNetwork() {
	nms := network.NewTestManagers(s.ctx, s.T(), 9100, 2)

	pool1, err := New(s.ctx, NewConfig(0), nms[0])
	s.Require().NoError(err)
	pool2, err := New(s.ctx, NewConfig(0), nms[1])
	s.Require().NoError(err)

	// Ensure that both nodes have subscribed, so that they will exchange this info on the following connect.
	s.Require().Eventually(func() bool {
		return slices.Contains(nms[0].PubSub().Topics(), topicPendingTransactions(0)) &&
			slices.Contains(nms[1].PubSub().Topics(), topicPendingTransactions(0))
	}, 1*time.Second, 50*time.Millisecond)

	network.ConnectManagers(s.T(), nms[0], nms[1])

	txn := newTransaction(defaultAddress, 0, 123)
	s.addTransactionsToPoolSuccessfully(pool1, txn)

	s.Eventually(func() bool {
		has, err := pool2.IdHashKnown(txn.Hash())
		s.Require().NoError(err)
		return has
	}, 20*time.Second, 200*time.Millisecond)
}

func (s *SuiteTxnPool) TestUnverifiedDuplicates() {
	txn1 := newTransaction(defaultAddress, 0, 123)
	txn2 := newTransaction(defaultAddress, 1, 123)

	s.addTransactionsSuccessfully(txn1, txn2)

	err := s.pool.Discard(s.ctx, []common.Hash{txn1.Hash(), txn2.Hash()}, DuplicateHash)
	s.Require().NoError(err)
}

func (s *SuiteTxnPool) checkTransactionsOrder(vals ...int) {
	s.T().Helper()

	txns := s.getTransactions()

	s.Require().Len(txns, len(vals))

	correct := true
	for i, txn := range txns {
		if int(txn.Value.Uint64()) != vals[i] {
			correct = false
			break
		}
	}
	if !correct {
		gotOrder := ""
		for _, txn := range txns {
			gotOrder += fmt.Sprintf("%d ", txn.Value.Uint64())
		}
		s.T().Errorf("Expected order: %v, got: [%v]", vals, gotOrder)
	}
}

func (s *SuiteTxnPool) getTransactionCount(pool Pool) int {
	s.T().Helper()

	txns, err := pool.Peek(1000000)
	s.Require().NoError(err)
	return len(txns)
}

func TestSuiteTxnpool(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteTxnPool))
}

func BenchmarkTxnPoolAdd(b *testing.B) {
	shardId := types.ShardId(0)
	ctx := b.Context()
	pool, err := New(ctx, NewConfig(shardId), nil)
	if err != nil {
		b.Fatalf("Failed to create transaction pool: %s", err)
	}

	pool.cfg.Size = uint64(b.N)
	zerolog.SetGlobalLevel(zerolog.Disabled)

	txns := make([]*types.Transaction, b.N)
	var addr types.Address
	for i := range b.N {
		if i%2 == 0 {
			addr = types.GenerateRandomAddress(shardId)
		}
		txns[i] = &types.Transaction{
			TransactionDigest: types.TransactionDigest{Seqno: types.Seqno(i), To: addr},
		}
	}

	b.ResetTimer()

	for i := range b.N {
		_, err = pool.Add(ctx, txns[i])
		if err != nil {
			b.Fatalf("Failed to add transaction to pool: %s", err)
		}
	}
}
