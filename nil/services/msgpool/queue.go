package msgpool

type MsgQueue struct {
	data []*metaMsg
}

func NewMessageQueue() *MsgQueue {
	return &MsgQueue{}
}

func (q *MsgQueue) Push(msg *metaMsg) {
	q.data = append(q.data, msg)
}

func (q *MsgQueue) Peek(n int) []*metaMsg {
	if len(q.data) < n {
		n = len(q.data)
	}
	return q.data[:n]
}

func (q *MsgQueue) Size() int {
	return len(q.data)
}

func (q *MsgQueue) Remove(msg *metaMsg) bool {
	for i, elem := range q.data {
		if elem.Seqno == msg.Seqno && elem.From.Equal(msg.From) {
			q.data = append(q.data[:i], q.data[i+1:]...)
			return true
		}
	}
	return false
}
