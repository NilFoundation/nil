package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
