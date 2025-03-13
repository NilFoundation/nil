package clickhouse

import (
	"testing"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
)

type TestBlock struct {
	Id uint64 `ch:"id"`
}

type TestStruct struct {
	Field1 int64         `ch:"field1"`
	Field2 string        `ch:"field2"`
	Field3 types.Uint256 `ch:"field3"`
	Block  TestBlock     `ch:"block"`
}

func TestReflectSchemeToClickhouse(t *testing.T) {
	t.Parallel()

	scheme, err := reflectSchemeToClickhouse(&TestBlock{})
	require.NoError(t, err)
	require.Equal(t, scheme.fieldNames["Id"], "id")
	require.Equal(t, scheme.fieldTypes["id"], "UInt64")
	require.Len(t, scheme.fieldTypes, 1)
}

func TestReflectComplexScheme(t *testing.T) {
	t.Parallel()

	scheme, err := reflectSchemeToClickhouse(&TestStruct{})
	require.NoError(t, err)
	require.Equal(t, scheme.fieldNames["Field1"], "field1")
	require.Equal(t, scheme.fieldTypes["field1"], "Int64")
	require.Equal(t, scheme.fieldNames["Field2"], "field2")
	require.Equal(t, scheme.fieldTypes["field2"], "String")
	require.Equal(t, scheme.fieldNames["Field3"], "field3")
	require.Equal(t, scheme.fieldTypes["field3"], "UInt256")
	require.Equal(t, scheme.fieldNames["Id"], "id")
	require.Equal(t, scheme.fieldTypes["id"], "UInt64")
}

func TestInitScheme(t *testing.T) {
	t.Parallel()
	require.NotPanics(t, func() {
		initSchemeCache()
	})
}
