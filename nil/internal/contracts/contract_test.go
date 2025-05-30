package contracts

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeCallData(t *testing.T) {
	t.Parallel()

	t.Run("SmartAccount", func(t *testing.T) {
		t.Parallel()

		saAbi, err := GetAbi(NameSmartAccount)
		require.NoError(t, err)

		data, err := saAbi.Pack("bounce", []byte("test string"))
		require.NoError(t, err)

		decoded, _, err := DecodeCallData(nil, data)
		require.NoError(t, err)
		require.Equal(t, "bounce(0x7465737420737472696e67)", decoded)
	})

	t.Run("tests/Test", func(t *testing.T) {
		t.Parallel()

		abi, err := GetAbi(NameTest)
		require.NoError(t, err)

		data, err := abi.Pack("emitLog", "test string", true)
		require.NoError(t, err)

		decoded, _, err := DecodeCallData(nil, data)
		require.NoError(t, err)
		require.Equal(t, "emitLog(test string, true)", decoded)
	})

	t.Run("system", func(t *testing.T) {
		t.Parallel()

		abi, err := GetAbi(NameL1BlockInfo)
		require.NoError(t, err)

		data, err := abi.Pack(
			"setL1BlockInfo", uint64(1), uint64(2), big.NewInt(3), big.NewInt(4), [32]byte{1, 2, 3, 4})
		require.NoError(t, err)

		decoded, _, err := DecodeCallData(nil, data)
		require.NoError(t, err)
		require.Equal(t,
			"setL1BlockInfo(1, 2, 3, 4, [1 2 3 4 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0])", decoded)
	})
}
