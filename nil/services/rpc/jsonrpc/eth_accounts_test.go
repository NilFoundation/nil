package jsonrpc

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

type SuiteAccountsBase struct {
	suite.Suite
	db        db.DB
	smcAddr   types.Address
	blockHash common.Hash
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

func (suite *SuiteEthAccounts) SetupSuite() {
	suite.SuiteAccountsBase.SetupSuite()

	shardId := types.BaseShardId
	ctx := suite.T().Context()

	var err error
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	es, err := execution.NewExecutionState(tx, shardId, execution.StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	suite.Require().NoError(err)
	es.BaseFee = types.DefaultGasPrice

	suite.smcAddr = types.GenerateRandomAddress(shardId)
	suite.Require().NotEmpty(suite.smcAddr)

	suite.Require().NoError(es.CreateAccount(suite.smcAddr))
	suite.Require().NoError(es.SetCode(suite.smcAddr, []byte("some code")))

	suite.Require().NoError(es.SetBalance(suite.smcAddr, types.NewValueFromUint64(1234)))
	suite.Require().NoError(es.SetExtSeqno(suite.smcAddr, 567))

	suite.Require().NoError(es.SetState(suite.smcAddr, common.HexToHash("0x1"), common.HexToHash("0x2")))
	suite.Require().NoError(es.SetState(suite.smcAddr, common.HexToHash("0x3"), common.HexToHash("0x4")))

	blockRes, err := es.Commit(0, nil)
	suite.Require().NoError(err)
	suite.blockHash = blockRes.BlockHash

	err = execution.PostprocessBlock(tx, shardId, blockRes, execution.ModeVerify)
	suite.Require().NotNil(blockRes.Block)
	suite.Require().NoError(err)

	err = tx.Commit()
	suite.Require().NoError(err)

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
	suite.Equal(hexutil.NewBigFromInt64(1234), res)

	blockHash := transport.BlockNumberOrHash{BlockHash: &suite.blockHash}
	res, err = suite.api.GetBalance(ctx, suite.smcAddr, blockHash)
	suite.Require().NoError(err)
	suite.Equal(hexutil.NewBigFromInt64(1234), res)

	blockNum = transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber}
	res, err = suite.api.GetBalance(ctx, types.GenerateRandomAddress(types.BaseShardId), blockNum)
	suite.Require().NoError(err)
	suite.True(res.IsZero())

	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	res, err = suite.api.GetBalance(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.True(res.IsZero())
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

	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	res, err = suite.api.GetCode(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
	suite.Empty(res)
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

	blockNumber := transport.BlockNumber(1000)
	blockNum = transport.BlockNumberOrHash{BlockNumber: &blockNumber}
	res, err = suite.api.GetTransactionCount(ctx, suite.smcAddr, blockNum)
	suite.Require().NoError(err)
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
	suite.EqualValues(1, resNonExisting.StorageProof[0].Key.Uint64())
	suite.EqualValues(0, resNonExisting.StorageProof[0].Value.Uint64())
	suite.Nil(resNonExisting.StorageProof[0].Proof)
	suite.EqualValues(2, resNonExisting.StorageProof[1].Key.Uint64())
	suite.EqualValues(0, resNonExisting.StorageProof[1].Value.Uint64())
	suite.Nil(resNonExisting.StorageProof[1].Proof)
}

// verifyProofResult verifies both account proof and storage proofs in the response
func (suite *SuiteEthAccounts) verifyProofResult(res *EthProof, smartContractsRoot common.Hash) {
	suite.T().Helper()

	// Verify account proof
	proof, err := mpt.SimpleProofFromBytesSlice(hexutil.ToBytesSlice(res.AccountProof))
	suite.Require().NoError(err)

	val, err := proof.Verify(smartContractsRoot, suite.smcAddr.Hash().Bytes())
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

	suite.Require().Equal(key.Big().Uint64(), storageProof.Key.Uint64())

	if expectNil {
		suite.Require().Equal(uint64(0), storageProof.Value.Uint64()) // no value for such key
	} else {
		var u types.Uint256
		suite.Require().NoError(u.UnmarshalNil(storageProof.Value.ToInt().Bytes()))
		suite.Require().Equal(expectedValue.Uint256(), u.Int())
	}

	proof, err := mpt.SimpleProofFromBytesSlice(hexutil.ToBytesSlice(storageProof.Proof))
	suite.Require().NoError(err)

	val, err := proof.Verify(storageRoot, key.Bytes())
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
