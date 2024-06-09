package msgpool

import (
	"bytes"
	"math"

	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/google/btree"
	"github.com/rs/zerolog"
)

// BySenderAndSeqno - designed to perform the most expensive operation in MsgPool:
// "recalculate all ephemeral fields of all transactions" by algo
//   - for all senders - iterate over all transactions in seqno growing order
//
// Performances decisions:
//   - All senders stored inside 1 large BTree - because iterate over 1 BTree is faster than over map[senderId]BTree
//   - sortByNonce used as non-pointer wrapper - because iterate over BTree of pointers is 2x slower
type BySenderAndSeqno struct {
	tree         *btree.BTreeG[*types.Message]
	search       *types.Message
	fromMsgCount map[types.Address]int // count of sender's msgs in the pool - may differ from seqno

	logger zerolog.Logger
}

func sortBySeqnoLess(a, b *types.Message) bool {
	fromCmp := bytes.Compare(a.From.Bytes(), b.From.Bytes())
	if fromCmp != 0 {
		return fromCmp == -1 // a < b
	}
	return a.Seqno < b.Seqno
}

func NewBySenderAndSeqno(logger zerolog.Logger) *BySenderAndSeqno {
	return &BySenderAndSeqno{
		tree:         btree.NewG(32, sortBySeqnoLess),
		search:       &types.Message{},
		fromMsgCount: map[types.Address]int{},
		logger:       logger,
	}
}

func (b *BySenderAndSeqno) seqno(from types.Address) (seqno uint64, ok bool) {
	s := b.search
	s.From = from
	s.Seqno = math.MaxUint64

	b.tree.DescendLessOrEqual(s, func(msg *types.Message) bool {
		if bytes.Equal(msg.From.Bytes(), from.Bytes()) {
			seqno = msg.Seqno
			ok = true
		}
		return false
	})
	return seqno, ok
}

func (b *BySenderAndSeqno) ascendAll(f func(*types.Message) bool) { //nolint:unused
	b.tree.Ascend(func(mt *types.Message) bool {
		return f(mt)
	})
}

func (b *BySenderAndSeqno) ascend(from types.Address, f func(*types.Message) bool) {
	s := b.search
	s.From = from
	s.Seqno = 0
	b.tree.AscendGreaterOrEqual(s, func(msg *types.Message) bool {
		if !bytes.Equal(msg.From.Bytes(), from.Bytes()) {
			return false
		}
		return f(msg)
	})
}

func (b *BySenderAndSeqno) descend(from types.Address, f func(*types.Message) bool) { //nolint:unused
	s := b.search
	s.From = from
	s.Seqno = math.MaxUint64
	b.tree.DescendLessOrEqual(s, func(msg *types.Message) bool {
		if !bytes.Equal(msg.From.Bytes(), from.Bytes()) {
			return false
		}
		return f(msg)
	})
}

func (b *BySenderAndSeqno) count(from types.Address) int { //nolint:unused
	return b.fromMsgCount[from]
}

func (b *BySenderAndSeqno) hasTxs(from types.Address) bool { //nolint:unused
	has := false
	b.ascend(from, func(*types.Message) bool {
		has = true
		return false
	})
	return has
}

func (b *BySenderAndSeqno) get(from types.Address, seqno uint64) *types.Message {
	s := b.search
	s.From = from
	s.Seqno = seqno
	if found, ok := b.tree.Get(s); ok {
		return found
	}
	return nil
}

func (b *BySenderAndSeqno) has(mt *types.Message) bool { //nolint:unused
	return b.tree.Has(mt)
}

func (b *BySenderAndSeqno) logTrace(msg *types.Message, logMsg string) {
	b.logger.Trace().
		Stringer(logging.FieldMessageHash, msg.Hash()).
		Stringer(logging.FieldMessageFrom, msg.From).
		Uint64(logging.FieldMessageSeqno, msg.Seqno).
		Msg(logMsg)
}

func (b *BySenderAndSeqno) delete(msg *types.Message, reason DiscardReason) {
	if _, ok := b.tree.Delete(msg); ok {
		b.logTrace(msg, "Deleted msg: "+reason.String())

		from := msg.From
		count := b.fromMsgCount[from]
		if count > 1 {
			b.fromMsgCount[from] = count - 1
		} else {
			delete(b.fromMsgCount, from)
		}
	}
}

func (b *BySenderAndSeqno) replaceOrInsert(msg *types.Message) *types.Message {
	it, ok := b.tree.ReplaceOrInsert(msg)
	if ok {
		b.logTrace(msg, "Replaced msg by seqno.")
		return it
	}

	b.logTrace(msg, "Inserted msg by seqno.")
	b.fromMsgCount[msg.From]++
	return nil
}
