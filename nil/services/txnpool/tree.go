package txnpool

import (
	"bytes"
	"math"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/google/btree"
)

// ByReceiverAndSeqno - designed to perform the most expensive operation in TxnPool:
// "recalculate all ephemeral fields of all transactions" by algo
//   - for all receivers - iterate over all transactions in seqno growing order
//
// Performances decisions:
//   - All senders stored inside 1 large BTree - because iterate over 1 BTree is faster than over map[senderId]BTree
//   - sortByNonce used as non-pointer wrapper - because iterate over BTree of pointers is 2x slower
type ByReceiverAndSeqno struct {
	tree       *btree.BTreeG[*metaTxn]
	search     *metaTxn
	toTxnCount map[types.Address]int // count of receiver's txns in the pool - may differ from seqno

	logger logging.Logger
}

func sortBySeqnoLess(a, b *metaTxn) bool {
	fromCmp := bytes.Compare(a.To.Bytes(), b.To.Bytes())
	if fromCmp != 0 {
		return fromCmp == -1 // a < b
	}
	return a.Seqno < b.Seqno
}

func NewBySenderAndSeqno(logger logging.Logger) *ByReceiverAndSeqno {
	return &ByReceiverAndSeqno{
		tree:       btree.NewG(32, sortBySeqnoLess),
		search:     &metaTxn{TxnWithHash: &types.TxnWithHash{Transaction: &types.Transaction{}}},
		toTxnCount: map[types.Address]int{},
		logger:     logger,
	}
}

func (b *ByReceiverAndSeqno) seqno(to types.Address) (seqno types.Seqno, ok bool) {
	s := b.search
	s.To = to
	s.Seqno = math.MaxUint64

	b.tree.DescendLessOrEqual(s, func(txn *metaTxn) bool {
		if txn.To.Equal(to) {
			seqno = txn.Seqno
			ok = true
		}
		return false
	})
	return seqno, ok
}

func (b *ByReceiverAndSeqno) ascendAll(f func(*metaTxn) bool) {
	b.tree.Ascend(f)
}

func (b *ByReceiverAndSeqno) ascend(to types.Address, f func(*metaTxn) bool) {
	s := b.search
	s.To = to
	s.Seqno = 0
	b.tree.AscendGreaterOrEqual(s, func(txn *metaTxn) bool {
		if !txn.To.Equal(to) {
			return false
		}
		return f(txn)
	})
}

func (b *ByReceiverAndSeqno) count(to types.Address) int { //nolint:unused
	return b.toTxnCount[to]
}

func (b *ByReceiverAndSeqno) hasTxs(to types.Address) bool { //nolint:unused
	has := false
	b.ascend(to, func(*metaTxn) bool {
		has = true
		return false
	})
	return has
}

func (b *ByReceiverAndSeqno) get(to types.Address, seqno types.Seqno) *metaTxn {
	s := b.search
	s.To = to
	s.Seqno = seqno
	if found, ok := b.tree.Get(s); ok {
		return found
	}
	return nil
}

func (b *ByReceiverAndSeqno) has(mt *metaTxn) bool { //nolint:unused
	return b.tree.Has(mt)
}

func (b *ByReceiverAndSeqno) logTrace(txn *metaTxn, format string, args ...any) {
	b.logger.Trace().
		Stringer(logging.FieldTransactionHash, txn.Hash()).
		Stringer(logging.FieldTransactionTo, txn.To).
		Uint64(logging.FieldTransactionSeqno, txn.Seqno.Uint64()).
		Msgf(format, args...)
}

func (b *ByReceiverAndSeqno) delete(txn *metaTxn, reason DiscardReason) {
	if _, ok := b.tree.Delete(txn); ok {
		b.logTrace(txn, "Deleted txn: %s", reason)

		to := txn.To
		count := b.toTxnCount[to]
		if count > 1 {
			b.toTxnCount[to] = count - 1
		} else {
			delete(b.toTxnCount, to)
		}
	}
}

func (b *ByReceiverAndSeqno) replaceOrInsert(txn *metaTxn) *metaTxn {
	it, ok := b.tree.ReplaceOrInsert(txn)
	if ok {
		b.logTrace(txn, "Replaced txn by seqno.")
		return it
	}

	b.logTrace(txn, "Inserted txn by seqno.")
	b.toTxnCount[txn.To]++
	return nil
}
