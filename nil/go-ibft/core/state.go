package core

import (
	"sync"

	"github.com/NilFoundation/nil/nil/go-ibft/messages"
	"github.com/NilFoundation/nil/nil/go-ibft/messages/proto"
)

type stateType uint8

const (
	newRound stateType = iota
	prepare
	commit
	fin
)

func (s stateType) String() string {
	switch s {
	case newRound:
		return "new round"
	case prepare:
		return "prepare"
	case commit:
		return "commit"
	case fin:
		return "fin"
	}

	return ""
}

type state struct {
	//	current view (sequence, round)
	view *proto.View

	// latestPC is the latest prepared certificate
	latestPC *proto.PreparedCertificate

	//	accepted block proposal for current round
	proposalMessage *proto.Message

	// latestPreparedProposedBlock is the block
	// for which Q(N)-1 PREPARE messages were received
	latestPreparedProposedBlock []byte

	//	validated commit seals
	seals []*messages.CommittedSeal

	//	flags for different states
	roundStarted bool

	// current state name
	name stateType

	sync.RWMutex
}

func (s *state) getView() *proto.View {
	s.RLock()
	defer s.RUnlock()

	return &proto.View{
		Height: s.view.Height,
		Round:  s.view.Round,
	}
}

func (s *state) clear(height uint64) {
	s.Lock()
	defer s.Unlock()

	s.seals = nil
	s.roundStarted = false
	s.name = newRound
	s.proposalMessage = nil
	s.latestPC = nil
	s.latestPreparedProposedBlock = nil

	s.view = &proto.View{
		Height: height,
		Round:  0,
	}
}

func (s *state) getLatestPC() *proto.PreparedCertificate {
	s.RLock()
	defer s.RUnlock()

	return s.latestPC
}

func (s *state) getLatestPreparedProposedBlock() []byte {
	s.RLock()
	defer s.RUnlock()

	return s.latestPreparedProposedBlock
}

func (s *state) getProposalMessage() *proto.Message {
	s.RLock()
	defer s.RUnlock()

	return s.proposalMessage
}

func (s *state) getProposalHash() []byte {
	s.RLock()
	defer s.RUnlock()

	return messages.ExtractProposalHash(s.proposalMessage)
}

func (s *state) setProposalMessage(proposalMessage *proto.Message) {
	s.Lock()
	defer s.Unlock()

	s.proposalMessage = proposalMessage
}

func (s *state) getRound() uint64 {
	s.RLock()
	defer s.RUnlock()

	return s.view.Round
}

func (s *state) getHeight() uint64 {
	s.RLock()
	defer s.RUnlock()

	return s.view.Height
}

func (s *state) getProposal() []byte {
	s.RLock()
	defer s.RUnlock()

	if s.proposalMessage != nil {
		return messages.ExtractProposal(s.proposalMessage)
	}

	return nil
}

func (s *state) getCommittedSeals() []*messages.CommittedSeal {
	s.RLock()
	defer s.RUnlock()

	return s.seals
}

func (s *state) getStateName() stateType {
	s.RLock()
	defer s.RUnlock()

	return s.name
}

func (s *state) changeState(name stateType) {
	s.Lock()
	defer s.Unlock()

	s.name = name
}

func (s *state) setRoundStarted(started bool) {
	s.Lock()
	defer s.Unlock()

	s.roundStarted = started
}

func (s *state) setView(view *proto.View) {
	s.Lock()
	defer s.Unlock()

	s.view = view
}

func (s *state) setCommittedSeals(seals []*messages.CommittedSeal) {
	s.Lock()
	defer s.Unlock()

	s.seals = seals
}

func (s *state) newRound() {
	s.Lock()
	defer s.Unlock()

	if !s.roundStarted {
		// Round is not yet started, kick the round off
		s.name = newRound
		s.roundStarted = true
	}
}

func (s *state) finalizePrepare(
	certificate *proto.PreparedCertificate,
	latestPPB []byte,
) {
	s.Lock()
	defer s.Unlock()

	s.latestPC = certificate
	s.latestPreparedProposedBlock = latestPPB

	// Move to the commit state
	s.name = commit
}
