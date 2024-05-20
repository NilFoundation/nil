package msgpool

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog"
)

type Pool interface {
	Add(ctx context.Context, newTxs []*types.Message, tx db.Tx) ([]DiscardReason, error)
	OnNewBlock(ctx context.Context, block *types.Block, committed []*types.Message, tx db.Tx) error
	// IdHashKnown check whether transaction with given Id hash is known to the pool
	IdHashKnown(tx db.Tx, hash common.Hash) (bool, error)
	Started() bool

	Peek(ctx context.Context, n int, onTopOf uint64) ([]*types.Message, error)
	SeqnoFromAddress(addr common.Address) (seqno uint64, inPool bool)
	MessageCount() int
	Get(tx db.Tx, hash common.Hash) (*types.Message, error)
}

type MsgPool struct {
	started bool
	cfg     Config

	lock          *sync.Mutex
	lastSeenCond  *sync.Cond
	lastSeenBlock atomic.Uint64

	byHash map[string]*types.Message // hash => msg : only those records not committed to db yet
	all    *BySenderAndSeqno         // from => (sorted map of msg seqno => *msg)
	queue  *MsgQueue
	logger *zerolog.Logger
}

func New(cfg Config) Pool {
	lock := &sync.Mutex{}

	return &MsgPool{
		started: true,
		cfg:     cfg,

		lock:         lock,
		lastSeenCond: sync.NewCond(lock),

		byHash: map[string]*types.Message{},
		all:    NewBySenderAndSeqno(),
		queue:  NewMessageQueue(),
		logger: common.NewLogger("msgpool", false),
	}
}

func (p *MsgPool) Add(ctx context.Context, msgs []*types.Message, tx db.Tx) ([]DiscardReason, error) {
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
			Stringer("hash", msg.Hash()).
			Stringer("from", msg.From).
			Msg("Added new message")
	}

	return discardReasons, nil
}

func (p *MsgPool) validateMsg(msg *types.Message) (DiscardReason, bool) {
	seqno, has := p.all.seqno(msg.From)
	if has && seqno > msg.Seqno {
		p.logger.Debug().
			Stringer("hash", msg.Hash()).
			Uint64("sender.seqno", seqno).
			Uint64("msg.seqno", msg.Seqno).
			Msg("seqno too low")
		return SeqnoTooLow, false
	}
	return NotSet, true
}

func (p *MsgPool) idHashKnownLocked(tx db.Tx, hash common.Hash) (bool, error) {
	if _, ok := p.byHash[string(hash.Bytes())]; ok {
		return true, nil
	}
	return false, nil
}

func (p *MsgPool) IdHashKnown(tx db.Tx, hash common.Hash) (bool, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.idHashKnownLocked(tx, hash)
}

func (p *MsgPool) Started() bool {
	return p.started
}

func (p *MsgPool) SeqnoFromAddress(addr common.Address) (seqno uint64, inPool bool) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.all.seqno(addr)
}

func (p *MsgPool) Get(tx db.Tx, hash common.Hash) (*types.Message, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.getLocked(tx, hash)
}

func (p *MsgPool) getLocked(tx db.Tx, hash common.Hash) (*types.Message, error) {
	msg, ok := p.byHash[string(hash.Bytes())]
	if ok {
		return msg, nil
	}
	return nil, nil
}

func (p *MsgPool) addLocked(msg *types.Message) DiscardReason {
	// Insert to pending pool, if pool doesn't have txn with same Nonce and bigger Tip
	found := p.all.get(msg.From, msg.Seqno)
	if found != nil {
		// Disard Message with lower fee (TODO: do we need it?)
		if found.Value.Cmp((*uint256.Int)(&msg.Value.Int)) >= 0 {
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

	if replaced := p.all.replaceOrInsert(msg, p.logger); replaced != nil {
		panic("must never happen")
	}

	p.queue.Push(msg)
	return NotSet
}

// dropping transaction from all sub-structures and from db
// Important: don't call it while iterating by "all"
func (p *MsgPool) discardLocked(msg *types.Message, reason DiscardReason) {
	hashStr := string(msg.Hash().Bytes())
	delete(p.byHash, hashStr)
	// p.deletedTxs = append(p.deletedTxs, mt)
	p.all.delete(msg, reason, p.logger)
}

func (p *MsgPool) OnNewBlock(ctx context.Context, block *types.Block, committed []*types.Message, tx db.Tx) (err error) {
	defer func() {
		p.logger.Debug().
			Int("committed", len(committed)).
			Int("queued", p.queue.Size()).
			Msg("New block")
	}()

	p.lock.Lock()
	defer func() {
		if err == nil {
			p.lastSeenBlock.Store(block.Id)
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
func (p *MsgPool) removeCommitted(bySeqno *BySenderAndSeqno, msgs []*types.Message) error {
	seqnosToRemove := map[common.Address]uint64{}
	for _, msg := range msgs {
		seqno, ok := seqnosToRemove[msg.From]
		if !ok || msg.Seqno > seqno {
			seqnosToRemove[msg.From] = msg.Seqno
		}
	}

	var toDel []*types.Message // can't delete items while iterate them

	discarded := 0

	for senderID, seqno := range seqnosToRemove {
		bySeqno.ascend(senderID, func(msg *types.Message) bool {
			if msg.Seqno > seqno {
				p.logger.Trace().
					Uint64("tx.seqno", msg.Seqno).
					Uint64("sender.seqno", seqno).
					Msg("Removing committed, cmp seqnos")

				return false
			}

			p.logger.Trace().
				Stringer("hash", msg.Hash()).
				Stringer("from", msg.From).
				Uint64("seqno", msg.Seqno).
				Msg("removeCommitted")

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
