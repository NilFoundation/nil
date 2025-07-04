package jsonrpc

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

type SuiteAccountsBase struct {
	suite.Suite
	db                 db.DB
	smcAddr            types.Address
	blockHash          common.Hash
	lastGeneratedBlock *types.Block
}

type SuiteEthAccounts struct {
	SuiteAccountsBase
	api *APIImpl
}

func (suite *SuiteAccountsBase) SetupSuite() {
	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)
}

func (suite *SuiteAccountsBase) TearDownSuite() {
	suite.db.Close()
}

// createAccount creates a new account with code, storage, and balance,
// commits the state (advances block number by 1), and returns the account address and block hash.
func (suite *SuiteAccountsBase) createAccount(
	storage map[common.Hash]common.Hash,
) (types.Address, common.Hash) {
	suite.T().Helper()

	shardId := types.BaseShardId

	ctx := suite.T().Context()
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	stateParams := execution.StateParams{}
	if suite.lastGeneratedBlock != nil {
		stateParams.Block = suite.lastGeneratedBlock
	}
	es := execution.NewTestExecutionState(suite.T(), tx, shardId, stateParams)

	suite.Require().NoError(err)
	es.BaseFee = types.DefaultGasPrice
	addr := types.GenerateRandomAddress(shardId)
	suite.Require().NotEmpty(addr)
	suite.Require().NoError(es.CreateAccount(addr))
	suite.Require().NoError(es.SetCode(addr, []byte("some code")))
	for k, v := range storage {
		suite.Require().NoError(es.SetState(addr, k, v))
	}

	suite.Require().NoError(es.SetBalance(addr, types.NewValueFromUint64(1234)))
	suite.Require().NoError(es.SetExtSeqno(addr, 567))

	blockRes, err := es.Commit(0, nil)
	suite.Require().NoError(err)

	err = execution.PostprocessBlock(tx, shardId, blockRes, execution.ModeVerify)
	suite.Require().NotNil(blockRes.Block)
	suite.Require().NoError(err)

	err = tx.Commit()
	suite.Require().NoError(err)

	suite.lastGeneratedBlock = blockRes.Block

	return addr, blockRes.BlockHash
}

func (suite *SuiteEthAccounts) SetupSuite() {
	suite.SuiteAccountsBase.SetupSuite()

	ctx := suite.T().Context()

	suite.smcAddr, suite.blockHash = suite.createAccount(map[common.Hash]common.Hash{
		common.HexToHash("0x01"): common.HexToHash("0x02"),
		common.HexToHash("0x03"): common.HexToHash("0x04"),
	})

	suite.api = NewTestEthAPI(ctx, suite.T(), suite.db, 2)
}

func (suite *SuiteEthAccounts) TearDownSuite() {
	suite.SuiteAccountsBase.TearDownSuite()
}

func (suite *SuiteEthAccounts) TestGetBalance() {
	ctx := suite.T().Context()

	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetBalance(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.EqualValues(1234, res.ToInt().Uint64())

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetBalance(ctx, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.EqualValues(1234, res.ToInt().Uint64())

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetBalance(ctx, types.GenerateRandomAddress(types.BaseShardId), blockNum)
	suite.Require().NoError(err)
	suite.Zero(res.ToInt().Uint64())

	// nonexistent block
	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	res, err = suite.api.GetBalance(ctx, suite.smcAddr, blockNum)
	suite.Require().ErrorIs(err, rawapitypes.ErrBlockNotFound)
	suite.Nil(res)
}

func (suite *SuiteEthAccounts) TestGetCode() {
	ctx := suite.T().Context()

	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetCode(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes("some code"), res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetCode(ctx, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Bytes("some code"), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetCode(ctx, types.GenerateRandomAddress(types.BaseShardId), blockNum)
	suite.Require().NoError(err)
	suite.Empty(res)

	// nonexistent block
	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	_, err = suite.api.GetCode(ctx, suite.smcAddr, blockNum)
	suite.Require().ErrorIs(err, rawapitypes.ErrBlockNotFound)
}

func (suite *SuiteEthAccounts) TestGetSeqno() {
	ctx := suite.T().Context()

	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err := suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(ctx, types.GenerateRandomAddress(types.BaseShardId), blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(0), res)

	// nonexistent block
	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	res, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().ErrorIs(err, rawapitypes.ErrBlockNotFound)
	suite.Equal(hexutil.Uint64(0), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.PendingBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(567), res)

	txn := types.ExternalTransaction{
		To:    suite.smcAddr,
		Seqno: 0,
	}

	key, err := crypto.GenerateKey()
	suite.Require().NoError(err)

	digest, err := txn.SigningHash()
	suite.Require().NoError(err)

	txn.AuthData, err = crypto.Sign(digest.Bytes(), key)
	suite.Require().NoError(err)

	data, err := txn.MarshalNil()
	suite.Require().NoError(err)

	hash, err := suite.api.SendRawTransaction(ctx, data)
	suite.Require().NoError(err)
	suite.NotEqual(common.EmptyHash, hash)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.PendingBlock.BlockNumber}
	res, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Equal(hexutil.Uint64(1), res)
}

func (suite *SuiteEthAccounts) TestGetProof() {
	ctx := suite.T().Context()

	// Test keys to check in proofs
	keys := []common.Hash{
		common.HexToHash("0x1"), // existing key
		common.HexToHash("0x2"), // non-existing key
	}

	// GetBlockByNumber response doesn't contain storage root, using rawapi method instead
	blockData, err := suite.api.rawapi.GetFullBlockData(ctx, suite.smcAddr.ShardId(),
		rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.LatestBlock))
	suite.Require().NoError(err)
	block, err := blockData.DecodeBytes()
	suite.Require().NoError(err)

	// Test with block number
	blockNum := transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	resByNum, err := suite.api.GetProof(ctx, suite.smcAddr, keys, blockNum)
	suite.Require().NoError(err)

	suite.verifyProofResult(resByNum, block.SmartContractsRoot)

	// Test with block hash
	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	resByHash, err := suite.api.GetProof(ctx, suite.smcAddr, keys, blockHash)
	suite.Require().NoError(err)

	suite.verifyProofResult(resByHash, block.SmartContractsRoot)

	// Test with non-existing address
	nonExistingAddr := types.GenerateRandomAddress(types.BaseShardId)
	resNonExisting, err := suite.api.GetProof(ctx, nonExistingAddr, keys, blockHash)
	suite.Require().NoError(err)

	// Verify empty result for non-existing address
	suite.Equal(nonExistingAddr, resNonExisting.Address)
	suite.NotEmpty(resNonExisting.AccountProof)
	suite.Zero(resNonExisting.Balance)
	suite.Zero(resNonExisting.CodeHash)
	suite.Zero(resNonExisting.Nonce)
	suite.Zero(resNonExisting.StorageHash)
	suite.Len(resNonExisting.StorageProof, 2)
	suite.EqualValues(1, resNonExisting.StorageProof[0].Key.ToInt().Uint64())
	suite.EqualValues(0, resNonExisting.StorageProof[0].Value.ToInt().Uint64())
	suite.Equal([]hexutil.Bytes{{0x80}}, resNonExisting.StorageProof[0].Proof)
	suite.EqualValues(2, resNonExisting.StorageProof[1].Key.ToInt().Uint64())
	suite.EqualValues(0, resNonExisting.StorageProof[1].Value.ToInt().Uint64())
	suite.Equal([]hexutil.Bytes{{0x80}}, resNonExisting.StorageProof[1].Proof)
}

// verifyProofResult verifies both account proof and storage proofs in the response
func (suite *SuiteEthAccounts) verifyProofResult(res *EthProof, smartContractsRoot common.Hash) {
	suite.T().Helper()

	// Verify account proof
	val, err := mpt.VerifyProof(smartContractsRoot,
		suite.smcAddr.Hash().Bytes(), toBytesSlice(res.AccountProof))
	suite.Require().NoError(err)

	var sc types.SmartContract
	suite.Require().NoError(sc.UnmarshalNil(val))

	// Verify account fields
	suite.Equal(suite.smcAddr, res.Address)
	suite.Require().Equal(res.Balance, sc.Balance)
	suite.Require().Equal(res.CodeHash, sc.CodeHash)
	suite.Require().Equal(res.Nonce, sc.Seqno)
	suite.Require().Equal(res.StorageHash, sc.StorageRoot)

	// Verify first storage proof (existing key)
	suite.verifyStorageProof(
		res.StorageProof[0],
		res.StorageHash,
		common.HexToHash("0x1"),
		common.HexToHash("0x2"),
		false,
	)

	// Verify second storage proof (non-existing key)
	suite.verifyStorageProof(
		res.StorageProof[1],
		res.StorageHash,
		common.HexToHash("0x2"),
		common.Hash{},
		true,
	)
}

// verifyStorageProof verifies an individual storage proof
func (suite *SuiteEthAccounts) verifyStorageProof(
	storageProof StorageProof,
	storageRoot common.Hash,
	key common.Hash,
	expectedValue common.Hash,
	expectNil bool,
) {
	suite.T().Helper()

	suite.Require().EqualValues(*key.Big(), storageProof.Key)

	if expectNil {
		suite.Require().Zero(storageProof.Value.ToInt().Uint64()) // no value for such key
	} else {
		suite.Require().Equal(expectedValue, common.BytesToHash(storageProof.Value.ToInt().Bytes()))
	}

	val, err := mpt.VerifyProof(storageRoot, key.Bytes(), toBytesSlice(storageProof.Proof))
	suite.Require().NoError(err)

	if expectNil {
		suite.Require().Nil(val)
	} else {
		var u types.Uint256
		suite.Require().NoError(u.UnmarshalNil(val))
		suite.Require().Equal(expectedValue.Uint256(), u.Int())
	}
}

func TestSuiteEthAccounts(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthAccounts))
}
