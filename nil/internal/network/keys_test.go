package network

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadOrGenerateKeys(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	fileName := tempDir + "/keys.yaml"

	privKey, err := LoadOrGenerateKeys(fileName)
	require.NoError(t, err)

	t.Run("load", func(t *testing.T) {
		t.Parallel()

		loaded, err := LoadOrGenerateKeys(fileName)
		require.NoError(t, err)

		require.Equal(t, privKey, loaded)
	})

	t.Run("new file", func(t *testing.T) {
		t.Parallel()

		newFileName := tempDir + "/new-keys.yaml"
		require.NotEqual(t, fileName, newFileName)

		generated, err := LoadOrGenerateKeys(newFileName)
		require.NoError(t, err)

		require.NotEqual(t, privKey, generated)
	})
}
