package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmptyCode(t *testing.T) {
	c := Code(nil)
	c.Hash()
}

func TestClone(t *testing.T) {
	c := Code("abcdef")
	c2 := c.Clone()
	(*c2.(*Code))[1] = 'B'
	require.Equal(t, byte('b'), c[1])
}
