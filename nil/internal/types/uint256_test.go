package types

import (
	"encoding/hex"
	"encoding/json"
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

	h, err := common.Keccak(value)
	require.NoError(t, err)
	assert.Equal(t, "3fa5449386ef45c61db8db3b8dba50dc4b899b5ea9afa02267d309fa92233c12", hex.EncodeToString(h[:]))
}

func TestUint256Json(t *testing.T) {
	t.Parallel()

	str, err := json.Marshal(*NewUint256(111))
	require.NoError(t, err)
	assert.JSONEq(t, "\"111\"", string(str))

	mapValue := map[Uint256]Uint256{
		*NewUint256(123): *NewUint256(321),
	}

	str, err = json.Marshal(mapValue)
	require.NoError(t, err)
	assert.JSONEq(t, `{"123":"321"}`, string(str))
}
