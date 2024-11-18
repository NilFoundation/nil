package tracer

import (
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/holiman/uint256"
)

type StackAccessor struct {
	stackData []uint256.Int
	curTopN   int
}

func NewStackAccessor(stackData []uint256.Int) *StackAccessor {
	return &StackAccessor{
		stackData,
		len(stackData) - 1,
	}
}

func (sa *StackAccessor) Pop() *uint256.Int {
	check.PanicIfNot(sa.curTopN >= 0)
	el := sa.stackData[sa.curTopN]
	sa.curTopN--
	return &el
}

func (sa *StackAccessor) PopUint64() uint64 {
	v := sa.Pop()
	return v.Uint64()
}

func (sa *StackAccessor) Back(n int) *uint256.Int {
	return &sa.stackData[sa.backIdx(n)]
}

func (sa *StackAccessor) PopWIndex() (*uint256.Int, int) {
	el, idx := sa.stackData[sa.curTopN], sa.curTopN
	sa.curTopN--
	return &el, idx
}

func (sa *StackAccessor) BackWIndex(n int) (*uint256.Int, int) {
	idx := sa.backIdx(n)
	return &sa.stackData[idx], idx
}

func (sa *StackAccessor) backIdx(n int) int {
	check.PanicIfNot(sa.curTopN >= n)
	return sa.curTopN - n
}

func (sa *StackAccessor) Skip(n int) {
	sa.curTopN -= min(n, sa.curTopN)
}

// Helper to track next rw operation counter (all operations should be sequentially
// ordered: (stack, memory, state).
type RwCounter struct {
	ctr uint
}

func (c *RwCounter) NextIdx() uint {
	ret := c.ctr
	c.ctr++
	return ret
}
