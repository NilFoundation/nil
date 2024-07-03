//go:build test

package collate

import (
	"context"

	"github.com/NilFoundation/nil/core/types"
)

type MockMsgPool struct {
	Msgs []*types.Message
}

var _ MsgPool = (*MockMsgPool)(nil)

func (m *MockMsgPool) Peek(context.Context, int, uint64) ([]*types.Message, error) {
	return m.Msgs, nil
}

func (m *MockMsgPool) OnNewBlock(context.Context, *types.Block, []*types.Message) error {
	return nil
}
