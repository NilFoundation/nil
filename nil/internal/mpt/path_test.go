package mpt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAt(t *testing.T) {
	t.Parallel()

	data := [2]byte{0x12, 0x34}
	nibbles := newPath(data[:], 0)
	require.Equal(t, 1, nibbles.At(0))
	require.Equal(t, 2, nibbles.At(1))
	require.Equal(t, 3, nibbles.At(2))
	require.Equal(t, 4, nibbles.At(3))
}

func TestAtWithOffset(t *testing.T) {
	t.Parallel()

	data := [2]byte{0x12, 0x34}
	nibbles := newPath(data[:], 1)
	require.Equal(t, 2, nibbles.At(0))
	require.Equal(t, 3, nibbles.At(1))
	require.Equal(t, 4, nibbles.At(2))
	assert.Panics(t, func() { nibbles.At(3) }, "Should panic")
}

func TestCommonPrefix(t *testing.T) {
	t.Parallel()

	nibblesA := newPath([]byte{0x12, 0x34}, 0)
	nibblesB := newPath([]byte{0x12, 0x56}, 0)
	common := nibblesA.CommonPrefix(nibblesB)
	require.True(t, common.Equal(newPath([]byte{0x12}, 0)))

	nibblesA = newPath([]byte{0x12, 0x34}, 0)
	nibblesB = newPath([]byte{0x12, 0x36}, 0)
	common = nibblesA.CommonPrefix(nibblesB)
	require.True(t, common.Equal(newPath([]byte{0x01, 0x23}, 1)))

	nibblesA = newPath([]byte{0x12, 0x34}, 1)
	nibblesB = newPath([]byte{0x12, 0x56}, 1)
	common = nibblesA.CommonPrefix(nibblesB)
	require.True(t, common.Equal(newPath([]byte{0x12}, 1)))

	nibblesA = newPath([]byte{0x52, 0x34}, 0)
	nibblesB = newPath([]byte{0x02, 0x56}, 0)
	common = nibblesA.CommonPrefix(nibblesB)
	require.True(t, common.Equal(newPath([]byte{}, 0)))
}

func TestCombine(t *testing.T) {
	t.Parallel()

	nibblesA := newPath([]byte{0x12, 0x34}, 0)
	nibblesB := newPath([]byte{0x56, 0x78}, 0)
	common := nibblesA.Combine(nibblesB)
	require.True(t, common.Equal(newPath([]byte{0x12, 0x34, 0x56, 0x78}, 0)))

	nibblesA = newPath([]byte{0x12, 0x34}, 1)
	nibblesB = newPath([]byte{0x56, 0x78}, 3)
	common = nibblesA.Combine(nibblesB)
	toCompare := newPath([]byte{0x23, 0x48}, 0)
	require.True(t, common.Equal(toCompare))
}
