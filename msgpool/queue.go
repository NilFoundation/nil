package msgpool

import (
	"bytes"

	"github.com/NilFoundation/nil/core/types"
)

type MsgQueue struct {
	data []*types.Message
}

func NewMessageQueue() *MsgQueue {
	return &MsgQueue{}
}

func (q *MsgQueue) Push(msg *types.Message) {
	q.data = append(q.data, msg)
}

func (q *MsgQueue) Peek(n int) []*types.Message {
	if len(q.data) < n {
		n = len(q.data)
	}
	return q.data[:n]
}

func (q *MsgQueue) Size() int {
	return len(q.data)
}

func (q *MsgQueue) Remove(msg *types.Message) bool {
	for i, elem := range q.data {
		if elem.Seqno == msg.Seqno && bytes.Equal(elem.From.Bytes(), msg.From.Bytes()) {
			q.data = append(q.data[:i], q.data[i+1:]...)
			return true
		}
	}
	return false
}
