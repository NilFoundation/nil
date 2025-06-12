package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

const (
	RangeResponseLimit = 256
)

type DebugAPI interface {
	GetBlockByNumber(
		ctx context.Context,
		shardId types.ShardId,
		number transport.BlockNumber,
		withTransactions bool,
	) (*DebugRPCBlock, error)
	GetBlockByHash(ctx context.Context, hash common.Hash, withTransactions bool) (*DebugRPCBlock, error)
	GetContract(
		ctx context.Context,
		contractAddr types.Address,
		blockNrOrHash transport.BlockNumberOrHash,
	) (*DebugRPCContract, error)
	GetBootstrapConfig(ctx context.Context) (*rpctypes.BootstrapConfig, error)
	AccountRange(
		ctx context.Context,
		shardId types.ShardId,
		blockNrOrHash transport.BlockNumberOrHash,
		start *common.Hash,
		maxResult uint64,
		noCode bool,
		noStorage bool,
		incompletes bool,
	) (*AccountsRange, error)
	StorageRangeAt(
		ctx context.Context,
		blockHash common.Hash,
		txId types.TransactionIndex,
		contractAddr types.Address,
		keyStart *common.Hash,
		maxResults uint,
	) (*StorageRange, error)
}

type DebugAPIImpl struct {
	logger logging.Logger
	rawApi rawapi.NodeApi
}

var _ DebugAPI = (*DebugAPIImpl)(nil)

func NewDebugAPI(rawApi rawapi.NodeApi, logger logging.Logger) *DebugAPIImpl {
	return &DebugAPIImpl{
		logger: logger,
		rawApi: rawApi,
	}
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *DebugAPIImpl) GetBlockByNumber(
	ctx context.Context,
	shardId types.ShardId,
	number transport.BlockNumber,
	withTransactions bool,
) (*DebugRPCBlock, error) {
	var blockReference rawapitypes.BlockReference
	if number <= 0 {
		switch number {
		case transport.LatestBlockNumber:
			blockReference = rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.LatestBlock)
		case transport.EarliestBlockNumber:
			blockReference = rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.EarliestBlock)
		case transport.LatestExecutedBlockNumber:
		case transport.FinalizedBlockNumber:
		case transport.SafeBlockNumber:
		case transport.PendingBlockNumber:
		default:
			return nil, fmt.Errorf("not supported special block number %s", number)
		}
	} else {
		blockReference = rawapitypes.BlockNumberAsBlockReference(types.BlockNumber(number))
	}
	return api.getBlockByReference(ctx, shardId, blockReference, withTransactions)
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *DebugAPIImpl) GetBlockByHash(
	ctx context.Context,
	hash common.Hash,
	withTransactions bool,
) (*DebugRPCBlock, error) {
	shardId := types.ShardIdFromHash(hash)
	return api.getBlockByReference(ctx, shardId, rawapitypes.BlockHashAsBlockReference(hash), withTransactions)
}

func suppressBlockNotFound(err error) error {
	if errors.Is(err, rawapitypes.ErrBlockNotFound) {
		return nil
	}
	return err
}

func (api *DebugAPIImpl) getBlockByReference(
	ctx context.Context,
	shardId types.ShardId,
	blockReference rawapitypes.BlockReference,
	withTransactions bool,
) (*DebugRPCBlock, error) {
	var blockData *types.RawBlockWithExtractedData
	var err error
	if withTransactions {
		blockData, err = api.rawApi.GetFullBlockData(ctx, shardId, blockReference)
		if err != nil {
			return nil, suppressBlockNotFound(err)
		}
	} else {
		blockHeader, err := api.rawApi.GetBlockHeader(ctx, shardId, blockReference)
		if err != nil {
			return nil, suppressBlockNotFound(err)
		}
		blockData = &types.RawBlockWithExtractedData{Block: blockHeader}
	}
	return EncodeRawBlockWithExtractedData(blockData)
}

func (api *DebugAPIImpl) GetContract(
	ctx context.Context,
	contractAddr types.Address,
	blockNrOrHash transport.BlockNumberOrHash,
) (*DebugRPCContract, error) {
	contract, err := api.rawApi.GetContract(ctx, contractAddr, toBlockReference(blockNrOrHash), true, true)
	if err != nil {
		return nil, err
	}

	return &DebugRPCContract{
		Contract:     contract.ContractBytes,
		Code:         hexutil.Bytes(contract.Code),
		Proof:        contract.ProofEncoded,
		Storage:      contract.Storage,
		Tokens:       contract.Tokens,
		AsyncContext: contract.AsyncContext,
	}, nil
}

func (api *DebugAPIImpl) GetBootstrapConfig(ctx context.Context) (*rpctypes.BootstrapConfig, error) {
	return api.rawApi.GetBootstrapConfig(ctx)
}

// AccountRange implements debug_accountRange
func (api *DebugAPIImpl) AccountRange(
	ctx context.Context,
	shardId types.ShardId,
	blockNrOrHash transport.BlockNumberOrHash,
	start *common.Hash,
	maxResults uint64,
	noCode bool,
	noStorage bool,
	incompletes bool,
) (*AccountsRange, error) {
	// In go-ethereum, the address is not stored as part of the value in the state trie,
	// so it can't be easily retrieved (i.e., you can't take the preimage of the key hash to get the address).
	// In our implementation, we store the address explicitly in the underlying structure,
	// so there's no need to track incomplete entries — the `incompletes` parameter is unused.
	_ = incompletes

	maxResults = min(maxResults, RangeResponseLimit)
	if start == nil {
		start = &common.EmptyHash
	}

	accountRange, err := api.rawApi.GetContractRange(
		ctx, shardId, toBlockReference(blockNrOrHash), *start, maxResults, !noCode, !noStorage,
	)
	if err != nil {
		return nil, err
	}
	accounts := make(map[types.Address]*RangedAccount, len(accountRange.Contracts))
	for _, rawAccount := range accountRange.Contracts {
		account, err := rangeAccountFromRawapi(rawAccount)
		if err != nil {
			return nil, err
		}
		accounts[account.Address] = account
	}

	blockHeaderRaw, err := api.rawApi.GetBlockHeader(ctx, shardId, toBlockReference(blockNrOrHash))
	if err != nil {
		return nil, err
	}

	var blockHeader types.Block
	if err := blockHeader.UnmarshalNil(blockHeaderRaw); err != nil {
		return nil, err
	}

	return &AccountsRange{
		Accounts: accounts,
		Next:     accountRange.Next,
		Root:     blockHeader.SmartContractsRoot,
	}, nil
}

// StorageRangeAt implements `debug_storageRangeAt` method. `txId` parameter is unsupported yet.
func (api *DebugAPIImpl) StorageRangeAt(
	ctx context.Context,
	blockHash common.Hash,
	txId types.TransactionIndex,
	contractAddr types.Address,
	keyStart *common.Hash,
	maxResults uint,
) (*StorageRange, error) {
	if txId != types.TransactionIndex(0) {
		return nil, errors.New("txId is unsupported")
	}

	maxResults = min(maxResults, RangeResponseLimit)

	contract, err := api.rawApi.GetContract(
		ctx, contractAddr, rawapitypes.BlockHashAsBlockReference(blockHash), true, false,
	)
	if err != nil {
		return nil, err
	}

	// Prepare storage data for trie
	keys, values := extractStorageKeyValues(contract.Storage)

	// Build storage trie
	trie, err := buildStorageTrie(keys, values)
	if err != nil {
		return nil, fmt.Errorf("failed to build storage trie: %w", err)
	}

	var ctr uint
	var nextKey *common.Hash
	storage := make(map[common.Hash]hexutil.Big)
	for key, value := range trie.IterateFromKey(keyStart.Bytes()) {
		if ctr == maxResults {
			nextHash := common.BytesToHash(key)
			nextKey = &nextHash
			break
		}
		var bigValue big.Int
		bigValue.SetBytes(value)
		storage[common.BytesToHash(key)] = hexutil.Big(bigValue)
		ctr++
	}
	return &StorageRange{Storage: storage, NextKey: nextKey}, nil
}

func rangeAccountFromRawapi(rawApiSc *rawapitypes.SmartContract) (*RangedAccount, error) {
	contract := new(types.SmartContract)
	if err := contract.UnmarshalNil(rawApiSc.ContractBytes); err != nil {
		return nil, err
	}
	acc := RangedAccount{
		Balance:     contract.Balance,
		Nonce:       contract.Seqno,
		StorageRoot: contract.StorageRoot,
		CodeHash:    contract.CodeHash,
		Code:        hexutil.Bytes(rawApiSc.Code),
		Address:     contract.Address,
		AddressHash: contract.Address.Hash(),
	}
	if rawApiSc.Storage != nil {
		hexutilStorage := make(map[common.Hash]hexutil.Big, len(rawApiSc.Storage))
		for k, v := range rawApiSc.Storage {
			hexutilStorage[k] = hexutil.Big(*v.ToBig())
		}
		acc.Storage = hexutilStorage
	}
	return &acc, nil
}

type RangedAccount struct {
	Balance     types.Value                 `json:"balance"`
	Nonce       types.Seqno                 `json:"nonce"`
	StorageRoot common.Hash                 `json:"root"`
	CodeHash    common.Hash                 `json:"codeHash"`
	Code        hexutil.Bytes               `json:"code,omitempty"`
	Storage     map[common.Hash]hexutil.Big `json:"storage,omitempty"`
	Address     types.Address               `json:"address"`
	AddressHash common.Hash                 `json:"key"`
}

type AccountsRange struct {
	Root     common.Hash                      `json:"root"`
	Accounts map[types.Address]*RangedAccount `json:"accounts"`
	// `Next` can be set to represent that this range is only partial, and `Next`
	// is where an iterator should be positioned in order to continue the range.
	Next *common.Hash `json:"next,omitempty"` // nil if no more accounts
}

// StorageRange is the result of a debug_storageRangeAt API call.
type StorageRange struct {
	Storage map[common.Hash]hexutil.Big `json:"storage"`
	NextKey *common.Hash                `json:"nextKey"` // nil if Storage includes the last key in the trie.
}
