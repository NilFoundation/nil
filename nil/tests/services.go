//go:build test

package tests

import (
	"context"
	"sync"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func StartFaucetService(t *testing.T, ctx context.Context, wg *sync.WaitGroup, client client.Client) (*faucet.Client, string) {
	t.Helper()

	endpoint := rpc.GetSockPathService(t, "faucet")

	serviceFaucet, err := faucet.NewService(client)
	require.NoError(t, err)

	wg.Add(1)
	go func() {
		defer wg.Done()
		assert.NoError(t, serviceFaucet.Run(ctx, endpoint))
	}()

	return faucet.NewClient(endpoint), endpoint
}
