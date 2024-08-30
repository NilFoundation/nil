package synccommittee

import (
	"bytes"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateStateUpdateTransaction(t *testing.T) {
	t.Parallel()

	p := NewProposer("", logging.NewLogger("sync_committee_aggregator_test"))

	provedStateRoot := common.IntToHash(10)
	newStateRoot := common.IntToHash(11)
	updateStateTransaction, err := p.createStateUpdateTransaction(provedStateRoot, newStateRoot)
	require.NoError(t, err)

	assert.Equal(t, uint64(0), updateStateTransaction.Nonce)
	assert.True(t, bytes.Contains(updateStateTransaction.Data, provedStateRoot.Bytes()))
	assert.True(t, bytes.Contains(updateStateTransaction.Data, newStateRoot.Bytes()))
}
