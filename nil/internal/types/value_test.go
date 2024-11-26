package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueJson(t *testing.T) {
	t.Parallel()

	str, err := json.Marshal(Value{})
	require.NoError(t, err)
	assert.JSONEq(t, "\"0\"", string(str))

	str, err = json.Marshal(NewZeroValue())
	require.NoError(t, err)
	assert.JSONEq(t, "\"0\"", string(str))

	str, err = json.Marshal(NewValueFromUint64(12345678))
	require.NoError(t, err)
	assert.JSONEq(t, "\"12345678\"", string(str))

	mapValue := map[Value]Value{
		NewValueFromUint64(123): NewZeroValue(),
		NewValueFromUint64(321): NewValueFromUint64(111),
		NewValueFromUint64(222): {},
	}

	str, err = json.Marshal(mapValue)
	require.NoError(t, err)
	assert.JSONEq(t, `{"123":"0", "321":"111", "222":"0"}`, string(str))
}
