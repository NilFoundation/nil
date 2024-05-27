package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type TestBlock struct {
	Id uint64
}

func TestReflectSchemeToClickhouse(t *testing.T) {
	t.Parallel()

	fields, err := ReflectSchemeToClickhouse(&TestBlock{})
	require.NoError(t, err)
	require.Contains(t, fields, "Id UInt64")
	require.Len(t, fields, 1)
}

func TestInitScheme(t *testing.T) {
	t.Parallel()
	require.NotPanics(t, func() {
		intiSchemeCache()
	})
}
