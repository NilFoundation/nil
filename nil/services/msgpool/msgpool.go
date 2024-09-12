package msgpool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
)

type Pool interface {
	Add(ctx context.Context, msgs ...*types.Message) ([]DiscardReason, error)
	OnCommitted(ctx context.Context, committed []*types.Message) error
	// IdHashKnown check whether transaction with given Id hash is known to the pool
	IdHashKnown(hash common.Hash) (bool, error)
	Started() bool

	Peek(ctx context.Context, n int) ([]*types.Message, error)
	SeqnoToAddress(addr types.Address) (seqno types.Seqno, inPool bool)
	MessageCount() int
	Get(hash common.Hash) (*types.Message, error)
}

type metaMsg struct {
	*types.Message
	hash common.Hash
}

func newMetaMsg(msg *types.Message) *metaMsg {
	return &metaMsg{
		Message: msg,
		hash:    msg.Hash(),
	}
}

type MsgPool struct {
	started bool
	cfg     Config

	networkManager *network.Manager

	lock sync.Mutex

	byHash map[string]*metaMsg // hash => msg : only those records not committed to db yet
	all    *ByReceiverAndSeqno // from => (sorted map of msg seqno => *msg)
	queue  *MsgQueue
	logger zerolog.Logger
}

func New(ctx context.Context, cfg Config, networkManager *network.Manager) (*MsgPool, error) {
	logger := logging.NewLogger("msgpool").With().
		Stringer(logging.FieldShardId, cfg.ShardId).
		Logger()

	res := &MsgPool{
		started: true,
		cfg:     cfg,

		networkManager: networkManager,

		byHash: map[string]*metaMsg{},
		all:    NewBySenderAndSeqno(logger),
		queue:  NewMessageQueue(),
		logger: logger,
	}

	if networkManager == nil {
		// we don't always want to run the network (e.g., in tests)
		return res, nil
	}

	sub, err := networkManager.PubSub().Subscribe(topicPendingMessages(cfg.ShardId))
	if err != nil {
		return nil, err
	}

	go func() {
		res.listen(ctx, sub)
	}()

	return res, nil
}

func (p *MsgPool) listen(ctx context.Context, sub *network.Subscription) {
	defer sub.Close()

	for m := range sub.Start(ctx) {
		msg := &types.Message{}
		if err := json.Unmarshal(m, msg); err != nil {
			p.logger.Error().Err(err).
				Msg("Failed to unmarshal message from network")
			continue
		}

		mm := newMetaMsg(msg)
		reasons, err := p.add(mm)
		if err != nil {
			p.logger.Error().Err(err).
				Stringer(logging.FieldMessageHash, mm.hash).
				Msg("Failed to add message from network")
			return
		}

		if reasons[0] != NotSet {
			p.logger.Debug().
				Stringer(logging.FieldMessageHash, mm.hash).
				Msgf("Discarded message from network with reason %s", reasons[0])
		}
	}
}

func (p *MsgPool) Add(ctx context.Context, msgs ...*types.Message) ([]DiscardReason, error) {
	mms := make([]*metaMsg, len(msgs))
	for i, msg := range msgs {
		mms[i] = newMetaMsg(msg)
	}

	reasons, err := p.add(mms...)
	if err != nil {
		return nil, err
	}

	for i, mm := range mms {
		if reasons[i] != NotSet {
			continue
		}

		if err := PublishPendingMessage(ctx, p.networkManager, p.cfg.ShardId, mm); err != nil {
			p.logger.Error().Err(err).
				Stringer(logging.FieldMessageHash, mm.hash).
				Msg("Failed to publish message to network")
		}
	}

	return reasons, nil
}

func (p *MsgPool) add(msgs ...*metaMsg) ([]DiscardReason, error) {
	discardReasons := make([]DiscardReason, len(msgs))

	p.lock.Lock()
	defer p.lock.Unlock()

	for i, msg := range msgs {
		if msg.To.ShardId() != p.cfg.ShardId {
			return nil, fmt.Errorf("message shard id %d does not match pool shard id %d", msg.To.ShardId(), p.cfg.ShardId)
		}

		if reason, ok := p.validateMsg(msg); !ok {
			discardReasons[i] = reason
			continue
		}

		if _, ok := p.byHash[string(msg.hash.Bytes())]; ok {
			discardReasons[i] = DuplicateHash
			continue
		}

		if reason := p.addLocked(msg); reason != NotSet {
			discardReasons[i] = reason
			continue
		}
		discardReasons[i] = NotSet // unnecessary
		p.logger.Debug().
			Uint64(logging.FieldShardId, uint64(msg.To.ShardId())).
			Stringer(logging.FieldMessageHash, msg.hash).
			Stringer(logging.FieldMessageTo, msg.To).
			Msg("Added new message.")
	}

	return discardReasons, nil
}

func (p *MsgPool) validateMsg(msg *metaMsg) (DiscardReason, bool) {
	seqno, has := p.all.seqno(msg.To)
	if has && seqno > msg.Seqno {
		p.logger.Debug().
			Uint64(logging.FieldShardId, uint64(msg.To.ShardId())).
			Stringer(logging.FieldMessageHash, msg.hash).
			Uint64(logging.FieldAccountSeqno, seqno.Uint64()).
			Uint64(logging.FieldMessageSeqno, msg.Seqno.Uint64()).
			Msg("Seqno too low.")
		return SeqnoTooLow, false
	}
	return NotSet, true
}

func (p *MsgPool) idHashKnownLocked(hash common.Hash) bool {
	if _, ok := p.byHash[string(hash.Bytes())]; ok {
		return true
	}
	return false
}

func (p *MsgPool) IdHashKnown(hash common.Hash) (bool, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.idHashKnownLocked(hash), nil
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
	msg := p.getLocked(hash)
	if msg == nil {
		return nil, nil
	}
	return msg.Message, nil
}

func (p *MsgPool) getLocked(hash common.Hash) *metaMsg {
	msg, ok := p.byHash[string(hash.Bytes())]
	if ok {
		return msg
	}
	return nil
}

func shouldReplace(existing, candidate *metaMsg) bool {
	if candidate.FeeCredit.Cmp(existing.FeeCredit) <= 0 {
		return false
	}

	// Discard the previous message if it is the same but at a lower fee
	existingFee := existing.FeeCredit
	existing.FeeCredit = candidate.FeeCredit
	defer func() {
		existing.FeeCredit = existingFee
	}()

	return bytes.Equal(existing.Hash().Bytes(), candidate.hash.Bytes())
}

func (p *MsgPool) addLocked(msg *metaMsg) DiscardReason {
	// Insert to pending pool, if pool doesn't have a txn with the same dst and seqno.
	// If pool has a txn with the same dst and seqno, only fee bump is possible; otherwise NotReplaced is returned.
	found := p.all.get(msg.To, msg.Seqno)
	if found != nil {
		if !shouldReplace(found, msg) {
			return NotReplaced
		}

		p.queue.Remove(found)
		p.discardLocked(found, ReplacedByHigherTip)
	}

	if uint64(p.queue.Size()) >= p.cfg.Size {
		return PoolOverflow
	}

	hashStr := string(msg.hash.Bytes())
	p.byHash[hashStr] = msg

	replaced := p.all.replaceOrInsert(msg)
	check.PanicIfNot(replaced == nil)

	p.queue.Push(msg)
	return NotSet
}

// dropping transaction from all sub-structures and from db
// Important: don't call it while iterating by "all"
func (p *MsgPool) discardLocked(mm *metaMsg, reason DiscardReason) {
	hashStr := string(mm.hash.Bytes())
	delete(p.byHash, hashStr)
	p.all.delete(mm, reason)
}

func (p *MsgPool) OnCommitted(_ context.Context, committed []*types.Message) (err error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.removeCommitted(p.all, committed)
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

	var toDel []*metaMsg // can't delete items while iterate them

	discarded := 0

	for senderID, seqno := range seqnosToRemove {
		bySeqno.ascend(senderID, func(msg *metaMsg) bool {
			if msg.Seqno > seqno {
				p.logger.Trace().
					Uint64(logging.FieldShardId, uint64(msg.To.ShardId())).
					Uint64(logging.FieldMessageSeqno, msg.Seqno.Uint64()).
					Uint64(logging.FieldAccountSeqno, seqno.Uint64()).
					Msg("Removing committed, cmp seqnos")

				return false
			}

			p.logger.Trace().
				Uint64(logging.FieldShardId, uint64(msg.To.ShardId())).
				Stringer(logging.FieldMessageHash, msg.hash).
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

func (p *MsgPool) Peek(ctx context.Context, n int) ([]*types.Message, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	mms := p.queue.Peek(n)
	msgs := make([]*types.Message, len(mms))
	for i, mm := range mms {
		msgs[i] = mm.Message
	}
	return msgs, nil
}

func (p *MsgPool) MessageCount() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.queue.Size()
}
