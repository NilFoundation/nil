//go:build test

package collate

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/types"
)

type MockMsgPool struct {
	Msgs []*types.Message
}

var _ MsgPool = (*MockMsgPool)(nil)

func (m *MockMsgPool) Peek(_ context.Context, n int) ([]*types.Message, error) {
	if n > len(m.Msgs) {
		return m.Msgs, nil
	}
	return m.Msgs[:n], nil
}

func (m *MockMsgPool) OnCommitted(context.Context, []*types.Message) error {
	return nil
}
