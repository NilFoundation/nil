package msgpool

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

type Pool interface {
	Add(ctx context.Context, newTxs []*types.Message) ([]DiscardReason, error)
	OnNewBlock(ctx context.Context, block *types.Block, committed []*types.Message) error
	// IdHashKnown check whether transaction with given Id hash is known to the pool
	IdHashKnown(hash common.Hash) (bool, error)
	Started() bool

	Peek(ctx context.Context, n int, onTopOf uint64) ([]*types.Message, error)
	SeqnoToAddress(addr types.Address) (seqno types.Seqno, inPool bool)
	MessageCount() int
	Get(hash common.Hash) (*types.Message, error)
}

type MsgPool struct {
	started bool
	cfg     Config

	lock          *sync.Mutex
	lastSeenCond  *sync.Cond
	lastSeenBlock atomic.Uint64

	byHash map[string]*types.Message // hash => msg : only those records not committed to db yet
	all    *ByReceiverAndSeqno       // from => (sorted map of msg seqno => *msg)
	queue  *MsgQueue
	logger zerolog.Logger
}

func New(cfg Config) *MsgPool {
	lock := &sync.Mutex{}

	logger := logging.NewLogger("msgpool")
	return &MsgPool{
		started: true,
		cfg:     cfg,

		lock:         lock,
		lastSeenCond: sync.NewCond(lock),

		byHash: map[string]*types.Message{},
		all:    NewBySenderAndSeqno(logger),
		queue:  NewMessageQueue(),
		logger: logger,
	}
}

func (p *MsgPool) Add(ctx context.Context, msgs []*types.Message) ([]DiscardReason, error) {
	discardReasons := make([]DiscardReason, len(msgs))

	p.lock.Lock()
	defer p.lock.Unlock()

	for i, msg := range msgs {
		if reason, ok := p.validateMsg(msg); !ok {
			discardReasons[i] = reason
			continue
		}

		if _, ok := p.byHash[string(msg.Hash().Bytes())]; ok {
			discardReasons[i] = DuplicateHash
			continue
		}

		if reason := p.addLocked(msg); reason != NotSet {
			discardReasons[i] = reason
			continue
		}
		discardReasons[i] = NotSet // unnecessary
		p.logger.Debug().
			Stringer(logging.FieldMessageHash, msg.Hash()).
			Stringer(logging.FieldMessageTo, msg.To).
			Msg("Added new message.")
	}

	return discardReasons, nil
}

func (p *MsgPool) validateMsg(msg *types.Message) (DiscardReason, bool) {
	seqno, has := p.all.seqno(msg.To)
	if has && seqno > msg.Seqno {
		p.logger.Debug().
			Stringer(logging.FieldMessageHash, msg.Hash()).
			Uint64(logging.FieldAccountSeqno, seqno.Uint64()).
			Uint64(logging.FieldMessageSeqno, msg.Seqno.Uint64()).
			Msg("Seqno too low.")
		return SeqnoTooLow, false
	}
	return NotSet, true
}

func (p *MsgPool) idHashKnownLocked(hash common.Hash) (bool, error) {
	if _, ok := p.byHash[string(hash.Bytes())]; ok {
		return true, nil
	}
	return false, nil
}

func (p *MsgPool) IdHashKnown(hash common.Hash) (bool, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.idHashKnownLocked(hash)
}

func (p *MsgPool) Started() bool {
	return p.started
}

func (p *MsgPool) SeqnoToAddress(addr types.Address) (seqno types.Seqno, inPool bool) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.all.seqno(addr)
}

func (p *MsgPool) Get(hash common.Hash) (*types.Message, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.getLocked(hash)
}

func (p *MsgPool) getLocked(hash common.Hash) (*types.Message, error) {
	msg, ok := p.byHash[string(hash.Bytes())]
	if ok {
		return msg, nil
	}
	return nil, nil
}

func (p *MsgPool) addLocked(msg *types.Message) DiscardReason {
	// Insert to pending pool, if pool doesn't have txn with the same Nonce and bigger Tip
	found := p.all.get(msg.To, msg.Seqno)
	if found != nil {
		// Discard Message with lower fee (TODO: do we need it?)
		if found.Value.Cmp(&msg.Value.Int) >= 0 {
			// TODO: Currently this condition can't be true because "Value" affects hash
			if bytes.Equal(found.Hash().Bytes(), msg.Hash().Bytes()) {
				return NotSet
			}
			return NotReplaced
		}

		p.queue.Remove(found)
		p.discardLocked(found, ReplacedByHigherTip)
	}

	if p.queue.Size() >= int(p.cfg.Size) {
		return PoolOverflow
	}

	hashStr := string(msg.Hash().Bytes())
	p.byHash[hashStr] = msg

	replaced := p.all.replaceOrInsert(msg)
	check.PanicIfNot(replaced == nil)

	p.queue.Push(msg)
	return NotSet
}

// dropping transaction from all sub-structures and from db
// Important: don't call it while iterating by "all"
func (p *MsgPool) discardLocked(msg *types.Message, reason DiscardReason) {
	hashStr := string(msg.Hash().Bytes())
	delete(p.byHash, hashStr)
	// p.deletedTxs = append(p.deletedTxs, mt)
	p.all.delete(msg, reason)
}

func (p *MsgPool) OnNewBlock(ctx context.Context, block *types.Block, committed []*types.Message) (err error) {
	p.lock.Lock()
	defer func() {
		p.logger.Debug().
			Int("committed", len(committed)).
			Int("queued", p.queue.Size()).
			Msg("New block")

		if err == nil {
			p.lastSeenBlock.Store(block.Id.Uint64())
			p.lastSeenCond.Broadcast()
		}

		p.lock.Unlock()
	}()

	if err = p.removeCommitted(p.all, committed); err != nil {
		return err
	}

	return nil
}

// removeCommitted - apply new highest block (or batch of blocks)
//
// 1. New block arrives, which potentially changes the balance and the seqno of some senders.
// We use senderIds data structure to find relevant senderId values, and then use senders data structure to
// modify state_balance and state_seqno, potentially remove some elements (if message with some seqno is
// included into a block), and finally, walk over the message records and update queue depending on
// the actual presence of seqno gaps and what the balance is.
func (p *MsgPool) removeCommitted(bySeqno *ByReceiverAndSeqno, msgs []*types.Message) error { //nolint:unparam
	seqnosToRemove := map[types.Address]types.Seqno{}
	for _, msg := range msgs {
		seqno, ok := seqnosToRemove[msg.To]
		if !ok || msg.Seqno > seqno {
			seqnosToRemove[msg.To] = msg.Seqno
		}
	}

	var toDel []*types.Message // can't delete items while iterate them

	discarded := 0

	for senderID, seqno := range seqnosToRemove {
		bySeqno.ascend(senderID, func(msg *types.Message) bool {
			if msg.Seqno > seqno {
				p.logger.Trace().
					Uint64(logging.FieldMessageSeqno, msg.Seqno.Uint64()).
					Uint64(logging.FieldAccountSeqno, seqno.Uint64()).
					Msg("Removing committed, cmp seqnos")

				return false
			}

			p.logger.Trace().
				Stringer(logging.FieldMessageHash, msg.Hash()).
				Stringer(logging.FieldMessageTo, msg.To).
				Uint64(logging.FieldMessageSeqno, msg.Seqno.Uint64()).
				Msg("Remove committed.")

			toDel = append(toDel, msg)
			p.queue.Remove(msg)
			return true
		})

		discarded += len(toDel)

		for _, msg := range toDel {
			p.discardLocked(msg, Committed)
		}
		toDel = toDel[:0]
	}

	if discarded > 0 {
		p.logger.Debug().
			Int("count", discarded).
			Msg("Discarded transactions")
	}

	return nil
}

func (p *MsgPool) Peek(ctx context.Context, n int, onTopOf uint64) ([]*types.Message, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for last := p.lastSeenBlock.Load(); last < onTopOf; last = p.lastSeenBlock.Load() {
		p.logger.Debug().
			Uint64("expecting", onTopOf).
			Uint64("lastSeen", last).
			Int("txRequested", n).
			Int("queue.size", p.queue.Size()).
			Msg("Waiting for block")
		p.lastSeenCond.Wait()
	}

	return p.queue.Peek(n), nil
}

func (p *MsgPool) MessageCount() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.queue.Size()
}
