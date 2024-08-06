package hexutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var encodeBytesTests = []marshalTest{
	{[]byte{}, "0x"},
	{[]byte{0}, "0x00"},
	{[]byte{0, 0, 1, 2}, "0x00000102"},
}

func TestEncode(t *testing.T) {
	t.Parallel()

	for _, test := range encodeBytesTests {
		in, ok := test.input.([]byte)
		require.True(t, ok)
		assert.Equal(t, test.want, Encode(in))
	}
}
