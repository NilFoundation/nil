package jsonrpc

import (
	"bytes"
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestDebugGetBlock(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer database.Close()

	tx, err := database.CreateRwTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()

	txn := types.NewEmptyTransaction()
	txnHash := txn.Hash()
	errStr := "test error"
	inTransactionTree := execution.NewDbTransactionTrie(tx, types.MainShardId)
	require.NoError(t, inTransactionTree.Update(types.TransactionIndex(0), txn))
	require.NoError(t, db.WriteError(tx, txnHash, errStr))
	_, err = inTransactionTree.Commit()
	require.NoError(t, err)

	blockWithErrors := &types.Block{
		BlockData: types.BlockData{
			Id:                 258,
			InTransactionsRoot: inTransactionTree.RootHash(),
		},
	}
	b1 := &execution.BlockGenerationResult{
		Block:       blockWithErrors,
		BlockHash:   blockWithErrors.Hash(types.MainShardId),
		InTxns:      []*types.Transaction{txn},
		InTxnHashes: []common.Hash{txnHash},
	}

	block := &types.Block{
		BlockData: types.BlockData{
			Id: 259,
		},
	}
	b2 := &execution.BlockGenerationResult{
		Block:     block,
		BlockHash: block.Hash(types.MainShardId),
	}

	var hexBytes []byte
	for _, b := range []*execution.BlockGenerationResult{b1, b2} {
		hexBytes, err = b.Block.MarshalNil()
		require.NoError(t, err)

		err = db.WriteBlock(tx, types.MainShardId, b.BlockHash, b.Block)
		require.NoError(t, err)

		err = execution.PostprocessBlock(tx, types.MainShardId, b, execution.ModeVerify)
		require.NoError(t, err)
	}

	err = tx.Commit()
	require.NoError(t, err)

	api := NewDebugAPI(
		rawapi.NodeApiBuilder(database, nil).
			WithLocalShardApiRo(types.MainShardId, nil).
			BuildAndReset(),
		logging.GlobalLogger)

	// When: Get the latest block
	res1, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.LatestBlockNumber, false)
	require.NoError(t, err)

	content := res1.Content
	require.EqualValues(t, hexBytes, content)

	// When: Get existing block
	res2, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.BlockNumber(block.Id), false)
	require.NoError(t, err)

	require.Equal(t, res1, res2)

	// When: Get nonexistent block
	res, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.BlockNumber(block.Id+1), false)
	require.NoError(t, err)
	require.Nil(t, res)

	// When: Get existing block with additional data
	res3, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.BlockNumber(blockWithErrors.Id), true)
	require.NoError(t, err)
	require.Len(t, res3.InTransactions, 1)
	require.Len(t, res3.Errors, 1)
	require.Equal(t, errStr, res3.Errors[txnHash])

	// When: Get existing block without additional data
	res4, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.BlockNumber(blockWithErrors.Id), false)
	require.NoError(t, err)
	require.Empty(t, res4.InTransactions)
}

type SuiteDbgContracts struct {
	SuiteAccountsBase
	debugApi  *DebugAPIImpl
	addresses []types.Address
}

func (suite *SuiteDbgContracts) SetupSuite() {
	suite.SuiteAccountsBase.SetupSuite()

	shardId := types.BaseShardId

	for range 2 {
		var addr types.Address
		addr, suite.blockHash = suite.createAccount(map[common.Hash]common.Hash{
			{0x01}: common.IntToHash(2),
			{0x03}: common.IntToHash(4),
		})
		suite.addresses = append(suite.addresses, addr)
	}
	// most of the tests will you this address
	suite.smcAddr = suite.addresses[0]

	suite.debugApi = NewDebugAPI(
		rawapi.NodeApiBuilder(suite.db, nil).
			WithLocalShardApiRo(shardId, nil).
			BuildAndReset(),
		logging.NewLogger("Test"))
}

func (suite *SuiteDbgContracts) TearDownSuite() {
	suite.SuiteAccountsBase.TearDownSuite()
}

func (suite *SuiteDbgContracts) TestGetContract() {
	ctx := context.Background()
	res, err := suite.debugApi.GetContract(
		ctx,
		suite.smcAddr,
		transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber})
	suite.Require().NoError(err)

	suite.Run("storage", func() {
		expected := map[common.Hash]types.Uint256{
			{0x1}: *types.NewUint256(2),
			{0x3}: *types.NewUint256(4),
		}
		suite.Require().Equal(expected, res.Storage)
	})

	suite.Run("proof", func() {
		tx, err := suite.db.CreateRoTx(ctx)
		suite.Require().NoError(err)
		defer tx.Rollback()

		shardId := suite.smcAddr.ShardId()
		block, err := db.ReadBlock(tx, shardId, suite.blockHash)
		suite.Require().NoError(err)

		contractRawReader := mpt.NewDbReader(tx, shardId, db.ContractTrieTable)
		suite.Require().NoError(contractRawReader.SetRootHash(block.SmartContractsRoot))

		expectedContract, err := contractRawReader.Get(suite.smcAddr.Hash().Bytes())
		suite.Require().NoError(err)

		proof, err := mpt.DecodeProof(res.Proof)
		suite.Require().NoError(err)

		ok, err := proof.VerifyRead(suite.smcAddr.Hash().Bytes(), expectedContract, block.SmartContractsRoot)
		suite.Require().NoError(err)
		suite.Require().True(ok)
	})
}

func (suite *SuiteDbgContracts) TestAccountRange() {
	ctx := context.Background()

	suite.Require().Len(suite.addresses, 2, "Test requires exactly two addresses")

	// Get and sort address hashes
	hash1 := suite.addresses[0].Hash()
	hash2 := suite.addresses[1].Hash()

	if bytes.Compare(hash2.Bytes(), hash1.Bytes()) < 0 {
		hash1, hash2 = hash2, hash1
		suite.addresses[0], suite.addresses[1] = suite.addresses[1], suite.addresses[0]
	}

	blockRef := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	shardID := suite.smcAddr.ShardId()

	// From nil (start of range)
	suite.Run("startFromNil", func() {
		res, err := suite.debugApi.AccountRange(ctx, shardID, blockRef, nil, 5, false, false, true)
		suite.Require().NoError(err)
		suite.Require().Len(res.Accounts, 2)
		suite.Require().Nil(res.Next)
	})

	// From first hash (inclusive)
	suite.Run("startFromFirstHash", func() {
		res, err := suite.debugApi.AccountRange(ctx, shardID, blockRef, &hash1, 5, false, false, true)
		suite.Require().NoError(err)
		suite.Require().Len(res.Accounts, 2)
		suite.Require().Nil(res.Next)
	})

	// From first hash + 1 (exclusive)
	suite.Run("startFromFirstHashPlusOne", func() {
		start := incrementHash(hash1)
		res, err := suite.debugApi.AccountRange(ctx, shardID, blockRef, &start, 5, false, false, true)
		suite.Require().NoError(err)
		suite.Require().Len(res.Accounts, 1)
		suite.Require().Equal(suite.addresses[1], res.Accounts[suite.addresses[1]].Address)
		suite.Require().Nil(res.Next)
	})

	// From second hash + 1 (should return nothing)
	suite.Run("startFromSecondHashPlusOne", func() {
		start := incrementHash(hash2)
		res, err := suite.debugApi.AccountRange(ctx, shardID, blockRef, &start, 5, false, false, true)
		suite.Require().NoError(err)
		suite.Require().Empty(res.Accounts)
		suite.Require().Nil(res.Next)
	})

	// Limit = 1
	suite.Run("limitOne", func() {
		res, err := suite.debugApi.AccountRange(ctx, shardID, blockRef, nil, 1, false, false, true)
		suite.Require().NoError(err)
		suite.Require().Len(res.Accounts, 1)
		suite.Require().NotNil(res.Next)
	})

	// Compare returned contract data with state
	suite.Run("accountEquality", func() {
		tx, err := suite.db.CreateRoTx(ctx)
		suite.Require().NoError(err)
		defer tx.Rollback()

		block, err := db.ReadBlock(tx, suite.smcAddr.ShardId(), suite.blockHash)
		suite.Require().NoError(err)

		contractTrieReader := execution.NewDbContractTrieReader(tx, shardID)
		suite.Require().NoError(contractTrieReader.SetRootHash(block.SmartContractsRoot))

		expectedContract, err := contractTrieReader.Fetch(suite.smcAddr.Hash())
		suite.Require().NoError(err)

		expectedCode, err := db.ReadCode(tx, shardID, expectedContract.CodeHash)
		suite.Require().NoError(err)

		// Query again to compare contents
		res, err := suite.debugApi.AccountRange(ctx, shardID, blockRef, nil, 5, false, false, true)
		suite.Require().NoError(err)
		suite.Require().Len(res.Accounts, 2)

		receivedContract, ok := res.Accounts[suite.smcAddr]
		suite.Require().True(ok, "Expected account not found in results")

		suite.Require().Equal(expectedContract.Balance, receivedContract.Balance)
		suite.Require().Equal(expectedContract.Seqno, receivedContract.Nonce)
		suite.Require().Equal(expectedContract.StorageRoot, receivedContract.StorageRoot)
		suite.Require().Equal(expectedContract.CodeHash, receivedContract.CodeHash)
		suite.Require().Equal(hexutil.Bytes(expectedCode), receivedContract.Code)
		suite.Require().NotEmpty(receivedContract.Storage)
		suite.Require().Equal(suite.smcAddr, receivedContract.Address)
		suite.Require().Equal(suite.smcAddr.Hash(), receivedContract.AddressHash)
	})
}

// addOne returns a new hash with value incremented by 1.
func incrementHash(h common.Hash) common.Hash {
	v := h.Uint256()
	v.Add(v, uint256.NewInt(1))
	return common.BigToHash(v.ToBig())
}

func TestSuiteDbgContracts(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteDbgContracts))
}
