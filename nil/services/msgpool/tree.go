package msgpool

import (
	"bytes"
	"math"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/google/btree"
	"github.com/rs/zerolog"
)

// ByReceiverAndSeqno - designed to perform the most expensive operation in MsgPool:
// "recalculate all ephemeral fields of all transactions" by algo
//   - for all receivers - iterate over all transactions in seqno growing order
//
// Performances decisions:
//   - All senders stored inside 1 large BTree - because iterate over 1 BTree is faster than over map[senderId]BTree
//   - sortByNonce used as non-pointer wrapper - because iterate over BTree of pointers is 2x slower
type ByReceiverAndSeqno struct {
	tree       *btree.BTreeG[*types.Message]
	search     *types.Message
	toMsgCount map[types.Address]int // count of receiver's msgs in the pool - may differ from seqno

	logger zerolog.Logger
}

func sortBySeqnoLess(a, b *types.Message) bool {
	fromCmp := bytes.Compare(a.To.Bytes(), b.To.Bytes())
	if fromCmp != 0 {
		return fromCmp == -1 // a < b
	}
	return a.Seqno < b.Seqno
}

func NewBySenderAndSeqno(logger zerolog.Logger) *ByReceiverAndSeqno {
	return &ByReceiverAndSeqno{
		tree:       btree.NewG(32, sortBySeqnoLess),
		search:     &types.Message{},
		toMsgCount: map[types.Address]int{},
		logger:     logger,
	}
}

func (b *ByReceiverAndSeqno) seqno(to types.Address) (seqno types.Seqno, ok bool) {
	s := b.search
	s.To = to
	s.Seqno = math.MaxUint64

	b.tree.DescendLessOrEqual(s, func(msg *types.Message) bool {
		if bytes.Equal(msg.To.Bytes(), to.Bytes()) {
			seqno = msg.Seqno
			ok = true
		}
		return false
	})
	return seqno, ok
}

func (b *ByReceiverAndSeqno) ascendAll(f func(*types.Message) bool) { //nolint:unused
	b.tree.Ascend(func(mt *types.Message) bool {
		return f(mt)
	})
}

func (b *ByReceiverAndSeqno) ascend(to types.Address, f func(*types.Message) bool) {
	s := b.search
	s.To = to
	s.Seqno = 0
	b.tree.AscendGreaterOrEqual(s, func(msg *types.Message) bool {
		if !bytes.Equal(msg.To.Bytes(), to.Bytes()) {
			return false
		}
		return f(msg)
	})
}

func (b *ByReceiverAndSeqno) descend(to types.Address, f func(*types.Message) bool) { //nolint:unused
	s := b.search
	s.To = to
	s.Seqno = math.MaxUint64
	b.tree.DescendLessOrEqual(s, func(msg *types.Message) bool {
		if !bytes.Equal(msg.To.Bytes(), to.Bytes()) {
			return false
		}
		return f(msg)
	})
}

func (b *ByReceiverAndSeqno) count(to types.Address) int { //nolint:unused
	return b.toMsgCount[to]
}

func (b *ByReceiverAndSeqno) hasTxs(to types.Address) bool { //nolint:unused
	has := false
	b.ascend(to, func(*types.Message) bool {
		has = true
		return false
	})
	return has
}

func (b *ByReceiverAndSeqno) get(to types.Address, seqno types.Seqno) *types.Message {
	s := b.search
	s.To = to
	s.Seqno = seqno
	if found, ok := b.tree.Get(s); ok {
		return found
	}
	return nil
}

func (b *ByReceiverAndSeqno) has(mt *types.Message) bool { //nolint:unused
	return b.tree.Has(mt)
}

func (b *ByReceiverAndSeqno) logTrace(msg *types.Message, logMsg string) {
	b.logger.Trace().
		Stringer(logging.FieldMessageHash, msg.Hash()).
		Stringer(logging.FieldMessageTo, msg.To).
		Uint64(logging.FieldMessageSeqno, msg.Seqno.Uint64()).
		Msg(logMsg)
}

func (b *ByReceiverAndSeqno) delete(msg *types.Message, reason DiscardReason) {
	if _, ok := b.tree.Delete(msg); ok {
		b.logTrace(msg, "Deleted msg: "+reason.String())

		to := msg.To
		count := b.toMsgCount[to]
		if count > 1 {
			b.toMsgCount[to] = count - 1
		} else {
			delete(b.toMsgCount, to)
		}
	}
}

func (b *ByReceiverAndSeqno) replaceOrInsert(msg *types.Message) *types.Message {
	it, ok := b.tree.ReplaceOrInsert(msg)
	if ok {
		b.logTrace(msg, "Replaced msg by seqno.")
		return it
	}

	b.logTrace(msg, "Inserted msg by seqno.")
	b.toMsgCount[msg.To]++
	return nil
}
