package types

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/constraints"
)

func test[T constraints.Integer](t *testing.T) {
	t.Helper()
	var b BitFlags[T]
	bitSize := b.BitsNum()

	for i := range bitSize {
		require.False(t, b.GetBit(i))
	}

	for i := range bitSize {
		b.SetBit(i)
		require.True(t, b.GetBit(i))
	}

	for i := 0; i < bitSize; i += 2 {
		b.ClearBit(i)
		require.False(t, b.GetBit(i))
	}

	for i := 0; i < bitSize; i += 2 {
		require.False(t, b.GetBit(i))
		require.True(t, b.GetBit(i+1))
	}

	b.Clear()
	for i := range bitSize {
		require.False(t, b.GetBit(i))
	}

	require.Panics(t, func() { b.SetBit(bitSize) })
	require.Panics(t, func() { b.SetBit(bitSize + 1) })

	flags := [4]int{1, 4, 5, bitSize - 1}
	b2 := NewBitFlags[T](flags[:]...)
	for i := range bitSize {
		if i == 1 || i == 4 || i == 5 || i == bitSize-1 {
			require.True(t, b2.GetBit(i))
		} else {
			require.False(t, b2.GetBit(i))
		}
	}

	b.Clear()

	require.True(t, b.None())

	err := b.SetRange(0, 5)
	require.NoError(t, err)
	require.Equal(t, uint64(0x1f), b.Uint64())

	b.Clear()
	err = b.SetRange(4, 3)
	require.NoError(t, err)
	require.Equal(t, uint64(0x70), b.Uint64())

	b.Clear()
	err = b.SetRange(0, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(0), b.Uint64())

	err = b.SetRange(math.MaxUint64, 5)
	require.Error(t, err)
	err = b.SetRange(1, uint(bitSize))
	require.Error(t, err)
}

func TestBitFlags(t *testing.T) {
	t.Parallel()

	test[uint8](t)
	test[uint16](t)
	test[uint32](t)
	test[uint64](t)
}
