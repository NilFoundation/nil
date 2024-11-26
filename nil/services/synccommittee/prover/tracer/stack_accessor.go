package tracer

import (
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
	el := sa.stackData[sa.curTopN]
	sa.curTopN--
	return &el
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
	return sa.curTopN - n
}

func (sa *StackAccessor) Skip(n int) {
	sa.curTopN -= n
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
