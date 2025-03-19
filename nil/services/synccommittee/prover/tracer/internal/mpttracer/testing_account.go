//go:build test

package mpttracer

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test account with tracer (they shared underlying MPT storage).
func CreateTestAccountAndTracer(t *testing.T) (types.Address, *MPTTracer, db.RwTx) {
	t.Helper()

	ctx := t.Context()
	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)

	rwTx, err := database.CreateRwTx(ctx)
	require.NoError(t, err)
	shardId := types.ShardId(0)
	storageTrie := execution.NewDbStorageTrie(rwTx, shardId)
	contractTrie := execution.NewDbContractTrie(rwTx, shardId)
	code := types.Code("")
	err = db.WriteCode(rwTx, shardId, code.Hash(), code) // without code `execution.NewAccountState` fails
	require.NoError(t, err)

	addr := types.GenerateRandomAddress(shardId)
	smartContract := types.SmartContract{
		Address:     addr,
		StorageRoot: storageTrie.RootHash(),
	}
	err = contractTrie.Update(smartContract.Address.Hash(), &smartContract)
	require.NoError(t, err)
	contractReader := &TestContractReader{
		RwTx:         rwTx,
		ContractTrie: contractTrie,
	}

	mptTracer := NewWithReader(contractReader, rwTx, shardId)

	return addr, mptTracer, rwTx
}

type TestContractReader struct {
	RwTx         db.RwTx
	ContractTrie *execution.BaseMPT[common.Hash, types.SmartContract, *types.SmartContract]
}

var _ ContractReader = (*TestContractReader)(nil)

func (tcr *TestContractReader) GetRwTx() db.RwTx {
	return tcr.RwTx
}

// AppendToJournal is no-op here, just to satisfy interface for account state
func (tcr *TestContractReader) AppendToJournal(je execution.JournalEntry) {
}

func (tcr *TestContractReader) GetAccount(
	_ context.Context,
	addr types.Address,
) (*types.SmartContract, mpt.Proof, error) {
	contract, err := tcr.ContractTrie.Fetch(addr.Hash())
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	proof, err := mpt.BuildProof(tcr.ContractTrie.Reader, addr.Hash().Bytes(), mpt.ReadMPTOperation)
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	return contract, proof, nil
}
