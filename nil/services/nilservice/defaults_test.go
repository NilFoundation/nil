package nilservice

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChainDefaults(t *testing.T) {
	t.Parallel()

	for chain, defaults := range DefaultsByChainName {
		t.Run(chain, func(t *testing.T) {
			t.Parallel()

			testChainDefaults(t, defaults)
		})
	}
}

func testChainDefaults(t *testing.T, defaults *ChainDefaults) {
	t.Helper()

	for _, p := range defaults.BootstrapAddresses {
		found := false
		for _, r := range defaults.RelayAddresses {
			if strings.HasPrefix(p, r) {
				found = true
				break
			}
		}
		assert.True(t, found, "bootstrap peer %s is not prefixed with any relay address", p)
	}
}
