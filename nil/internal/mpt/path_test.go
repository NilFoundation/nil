package mpt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAt(t *testing.T) {
	t.Parallel()

	data := [2]byte{0x12, 0x34}
	nibbles := newPath(data[:], false)
	require.Equal(t, 1, nibbles.At(0))
	require.Equal(t, 2, nibbles.At(1))
	require.Equal(t, 3, nibbles.At(2))
	require.Equal(t, 4, nibbles.At(3))
}

func TestAtWithOffset(t *testing.T) {
	t.Parallel()

	data := [2]byte{0x12, 0x34}
	nibbles := newPath(data[:], true)
	require.Equal(t, 2, nibbles.At(0))
	require.Equal(t, 3, nibbles.At(1))
	require.Equal(t, 4, nibbles.At(2))
	assert.Panics(t, func() { nibbles.At(3) }, "Should panic")
}

func TestCommonPrefix(t *testing.T) {
	t.Parallel()

	nibblesA := newPath([]byte{0x12, 0x34}, false)
	nibblesB := newPath([]byte{0x12, 0x56}, false)
	common := nibblesA.CommonPrefix(nibblesB)
	require.True(t, common.Equal(newPath([]byte{0x12}, false)))

	nibblesA = newPath([]byte{0x12, 0x34}, false)
	nibblesB = newPath([]byte{0x12, 0x36}, false)
	common = nibblesA.CommonPrefix(nibblesB)
	require.True(t, common.Equal(newPath([]byte{0x01, 0x23}, true)))

	nibblesA = newPath([]byte{0x12, 0x34}, true)
	nibblesB = newPath([]byte{0x12, 0x56}, true)
	common = nibblesA.CommonPrefix(nibblesB)
	require.True(t, common.Equal(newPath([]byte{0x12}, true)))

	nibblesA = newPath([]byte{0x52, 0x34}, false)
	nibblesB = newPath([]byte{0x02, 0x56}, false)
	common = nibblesA.CommonPrefix(nibblesB)
	require.True(t, common.Equal(newPath([]byte{}, false)))
}

func TestCombine(t *testing.T) {
	t.Parallel()

	nibblesA := newPath([]byte{0x12, 0x34}, false)
	nibblesB := newPath([]byte{0x56, 0x78}, false)
	common := nibblesA.Combine(nibblesB)
	require.True(t, common.Equal(newPath([]byte{0x12, 0x34, 0x56, 0x78}, false)))

	nibblesA = newPath([]byte{0x12, 0x34}, true)
	nibblesB = newPath([]byte{0x78}, true)
	common = nibblesA.Combine(nibblesB)
	toCompare := newPath([]byte{0x23, 0x48}, false)
	require.True(t, common.Equal(toCompare))
}

func TestConsume(t *testing.T) {
	t.Parallel()

	nibbles := newPath([]byte{0x12, 0x34}, false)
	nibbles.Consume(1)
	require.True(t, nibbles.Equal(newPath([]byte{0x2, 0x34}, true)))

	nibbles = newPath([]byte{0x1, 0x23, 0x45}, true)
	nibbles.Consume(2)
	require.True(t, nibbles.Equal(newPath([]byte{0x3, 0x45}, true)))
}
