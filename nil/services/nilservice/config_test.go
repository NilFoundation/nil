package nilservice

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := NewDefaultConfig()
	require.NoError(t, cfg.Validate())
}

func TestValidateInvalidMyShards(t *testing.T) {
	t.Parallel()

	cfg := NewDefaultConfig()
	cfg.MyShards = []uint{100}

	err := cfg.Validate()
	require.ErrorContains(t, err, "Shard 100 is out of range (nShards = 5)")
}

func TestValidateInvalidNShards(t *testing.T) {
	t.Parallel()

	cfg := NewDefaultConfig()

	cfg.NShards = 0
	err := cfg.Validate()
	require.ErrorContains(t, err, "NShards must be greater than 2 (main shard + 1)")

	cfg.NShards = 1
	err = cfg.Validate()
	require.ErrorContains(t, err, "NShards must be greater than 2 (main shard + 1)")

	cfg.NShards = 2
	require.NoError(t, cfg.Validate())
}

func TestValidateInvalidGasPriceScale(t *testing.T) {
	t.Parallel()

	cfg := NewDefaultConfig()
	cfg.GasPriceScale = -5

	err := cfg.Validate()
	require.ErrorContains(t, err, "GasPriceScale must be >= 0")
}
