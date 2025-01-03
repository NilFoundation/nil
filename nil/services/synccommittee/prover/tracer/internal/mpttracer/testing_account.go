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

// Helper function to create a test account with storage trie.
func CreateTestAccount(t *testing.T) (types.Address, *MPTTracer) {
	t.Helper()

	ctx := context.Background()
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

	return addr, mptTracer
}

type TestContractReader struct {
	RwTx         db.RwTx
	ContractTrie *execution.BaseMPT[common.Hash, types.SmartContract, *types.SmartContract]
}

func (tcr *TestContractReader) GetRwTx() db.RwTx {
	return tcr.RwTx
}

// AppendToJournal is no-op here, just to satisfy iterface for account state
func (tcr *TestContractReader) AppendToJournal(je execution.JournalEntry) {
}

func (tcr *TestContractReader) GetAccount(_ context.Context, addr types.Address) (*TracerAccount, mpt.Proof, error) {
	contract, err := tcr.ContractTrie.Fetch(addr.Hash())
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	tracerAccount, err := NewTracerAccount(tcr, contract.Address, contract)
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	proof, err := mpt.BuildProof(tcr.ContractTrie.Reader, addr.Hash().Bytes(), mpt.ReadMPTOperation)
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	return tracerAccount, proof, nil
}

var _ ContractReader = &TestContractReader{}
