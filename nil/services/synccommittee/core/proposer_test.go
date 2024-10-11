package core

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ChainId          = "11155111"
	ContractAddress  = "0xB8E280a085c87Ed91dd6605480DD2DE9EC3699b4"
	FunctionSelector = "0x6af78c5c"
)

func newTestProposer(t *testing.T) *Proposer {
	t.Helper()
	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	logger := logging.NewLogger("sync_committee_aggregator_test")
	blockStorage := storage.NewBlockStorage(database, logger)
	proposer, err := NewProposer(DefaultProposerParams(), blockStorage, logger)
	require.NoError(t, err)
	return proposer
}

func TestCreateUpdateStateTransaction(t *testing.T) {
	t.Parallel()

	p := newTestProposer(t)

	provedStateRoot := common.IntToHash(10)
	newStateRoot := common.IntToHash(11)
	updateStateTransaction, err := p.createUpdateStateTransaction(provedStateRoot, newStateRoot)
	require.NoError(t, err)

	assert.Equal(t, p.seqno.Load(), updateStateTransaction.Nonce())

	chainId, ok := new(big.Int).SetString(ChainId, 10)
	assert.True(t, ok)
	assert.Equal(t, chainId, updateStateTransaction.ChainId())

	assert.Equal(t, ethcommon.HexToAddress(ContractAddress), *updateStateTransaction.To())

	// check Data
	functionSelector, err := hexutil.Decode(FunctionSelector)
	require.NoError(t, err)
	assert.True(t, bytes.Contains(updateStateTransaction.Data(), functionSelector))
	assert.True(t, bytes.Contains(updateStateTransaction.Data(), provedStateRoot.Bytes()))
	assert.True(t, bytes.Contains(updateStateTransaction.Data(), newStateRoot.Bytes()))
}
