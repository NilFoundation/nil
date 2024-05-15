package types

import (
	"fmt"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/ssz"
)

const TopicsLimit = 1000000

type Topic common.Hash

var _ ssz.SizedObjectSSZ = (*Topic)(nil)

func (h *Topic) EncodeSSZ(dst []byte) ([]byte, error) {
	if len(h) != common.HashSize {
		return nil, fmt.Errorf("Topic length must be %d bytes", common.HashSize)
	}
	return append(dst, h[:]...), nil
}

func (h *Topic) EncodingSizeSSZ() int {
	return common.HashSize
}

func (h *Topic) DecodeSSZ(buf []byte, version int) error {
	if len(buf) < common.HashSize {
		return fmt.Errorf("hash length must be %d bytes", common.HashSize)
	}
	copy(h[:], buf)
	return nil
}

func (h *Topic) Static() bool {
	return false
}

func (h *Topic) Clone() common.Clonable {
	if h == nil {
		return &Topic{}
	}
	clonned := *h
	return &clonned
}

type Log struct {
	// Address of the contract that generated the event
	Address common.Address
	// List of topics provided by the contract
	Topics *ssz.ListSSZ[*Topic]
	// Supplied by the contract, usually ABI-encoded
	Data []byte

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber uint64
}

var _ ssz.ObjectSSZ = (*Log)(nil)

type Logs []*Log

func NewLog(address common.Address, data []byte, blockNumber uint64, topics []*Topic) *Log {
	l := new(Log)
	l.Address = address
	l.Data = data
	l.BlockNumber = blockNumber
	l.Topics = ssz.NewDynamicListSSZ[*Topic](TopicsLimit)
	l.SetTopics(topics)
	return l
}

func (l *Log) SetTopics(topics []*Topic) {
	for _, t := range topics {
		l.Topics.Append(t)
	}
}

func (l *Log) EncodeSSZ(buf []byte) (res []byte, err error) {
	buf, err = ssz.MarshalSSZ(buf, l.Address[:], l.Topics, (*ssz.Vector)(&l.Data))
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func (l *Log) EncodingSizeSSZ() int {
	size := 0
	size += common.AddrSize
	size += 4 + l.Topics.EncodingSizeSSZ()
	size += 4 + len(l.Data)
	return size
}

func (l *Log) DecodeSSZ(buf []byte, version int) error {
	if err := ssz.UnmarshalSSZ(buf, 0, l.Address[:], l.Topics, (*ssz.Vector)(&l.Data)); err != nil {
		return err
	}
	return nil
}

func (l *Log) Clone() common.Clonable {
	if l == nil {
		log := new(Log)
		log.Topics = ssz.NewDynamicListSSZ[*Topic](TopicsLimit)
		return log
	}
	cloned := *l
	return &cloned
}
