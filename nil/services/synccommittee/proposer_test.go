package synccommittee

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	L1Endpoint       = "http://rpc2.sepolia.org"
	ChainId          = "11155111"
	PrivateKey       = "0000000000000000000000000000000000000000000000000000000000000001"
	ContractAddress  = "0xB8E280a085c87Ed91dd6605480DD2DE9EC3699b4"
	FunctionSelector = "0x6af78c5c"
)

func TestCreateUpdateStateTransaction(t *testing.T) {
	t.Parallel()

	p, err := NewProposer(L1Endpoint, ChainId, PrivateKey, ContractAddress, logging.NewLogger("sync_committee_aggregator_test"))
	require.NoError(t, err)

	provedStateRoot := common.IntToHash(10)
	newStateRoot := common.IntToHash(11)
	updateStateTransaction, err := p.createUpdateStateTransaction(provedStateRoot, newStateRoot)
	require.NoError(t, err)

	assert.Equal(t, uint64(0), updateStateTransaction.Nonce())

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

func TestSendProof(t *testing.T) {
	t.Parallel()

	p, err := NewProposer(L1Endpoint, ChainId, PrivateKey, ContractAddress, logging.NewLogger("sync_committee_aggregator_test"))
	require.NoError(t, err)

	provedStateRoot := common.IntToHash(10)
	newStateRoot := common.IntToHash(11)
	transactions := make([]*prunedTransaction, 0)
	err = p.SendProof(provedStateRoot, newStateRoot, transactions)
	require.NoError(t, err)
}
