package types

import (
	"encoding/hex"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUint256SSZ(t *testing.T) {
	t.Parallel()

	value := NewUint256(102030)

	v, err := value.MarshalSSZTo([]byte{})
	require.NoError(t, err)
	assert.Equal(t, "8e8e010000000000000000000000000000000000000000000000000000000000", hex.EncodeToString(v))

	v, err = value.MarshalSSZTo([]byte{1, 2})
	require.NoError(t, err)
	assert.Equal(t, "01028e8e010000000000000000000000000000000000000000000000000000000000", hex.EncodeToString(v))

	res, err := value.HashTreeRoot()
	require.NoError(t, err)
	assert.Equal(t, "8e8e010000000000000000000000000000000000000000000000000000000000", hex.EncodeToString(res[:]))

	h, err := common.PoseidonSSZ(value)
	require.NoError(t, err)
	assert.Equal(t, "0912604ab702e08cf1173ee710b035d3efae416bf8ebb5fccb04a0fc8cc5d1a0", hex.EncodeToString(h[:]))
}
