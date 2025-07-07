package internal

import (
	"context"
	"errors"
	"fmt"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"math/big"
)

func (api *localShardApiRo) errWrongShard() error {
	return fmt.Errorf("address is not in the shard %d", api.shard)
}

func (api *localShardApiRo) GetBalance(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (types.Value, error) {
	shardId := address.ShardId()
	if shardId != api.shardId() {
		return types.Value{}, api.errWrongShard()
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return types.Value{}, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, _, err := api.getSmartContract(tx, address, blockReference, false)
	if err != nil {
		return types.Value{}, err
	}
	if acc == nil {
		return types.Value{}, nil
	}
	return acc.Balance, nil
}

func (api *localShardApiRo) GetCode(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (types.Code, error) {
	shardId := address.ShardId()
	if shardId != api.shardId() {
		return types.Code{}, api.errWrongShard()
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, _, err := api.getSmartContract(tx, address, blockReference, false)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, nil
	}

	code, err := db.ReadCode(tx, shardId, acc.CodeHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return code, nil
}

type token struct {
	Token   types.Address
	Balance *big.Int
}

func (api *localShardApi) GetTokens(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (map[types.TokenId]types.Value, error) {
	abi, err := contracts.GetAbi(contracts.NameTokenManager)
	if err != nil {
		return nil, fmt.Errorf("cannot get ABI: %w", err)
	}

	calldata, err := abi.Pack("getTokens", address)
	if err != nil {
		return nil, fmt.Errorf("cannot pack calldata: %w", err)
	}

	tokenManagerAddr := types.ShardAndHexToAddress(address.ShardId(), types.TokenManagerPureAddress)

	ret, err := api.CallGetter(ctx, tokenManagerAddr, calldata)
	if err != nil {
		return nil, fmt.Errorf("failed to call getter: %w", err)
	}

	var tokens []token
	err = abi.UnpackIntoInterface(&tokens, "getTokens", ret)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack response: %w", err)
	}

	res := make(map[types.TokenId]types.Value)
	for t := range tokens {
		res[types.TokenId(tokens[t].Token)] = types.NewValueFromBigMust(tokens[t].Balance)
	}
	return res, nil
}

func (api *localShardApi) CallGetter(
	ctx context.Context,
	address types.Address,
	calldata []byte,
) ([]byte, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	block, _, err := db.ReadLastBlock(tx, address.ShardId())
	if err != nil {
		return nil, fmt.Errorf("failed to read last block: %w", err)
	}

	cfgAccessor, err := config.NewConfigReader(tx, &block.MainShardHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create config accessor: %w", err)
	}

	es, err := execution.NewExecutionState(tx, address.ShardId(), execution.StateParams{
		Block:          block,
		ConfigAccessor: cfgAccessor,
		Mode:           execution.ModeReadOnly,
	})
	if err != nil {
		return nil, err
	}

	extTxn := &types.ExternalTransaction{
		FeeCredit:    types.GasToValue(types.DefaultMaxGasInBlock.Uint64()),
		MaxFeePerGas: types.MaxFeePerGasDefault,
		To:           address,
		Data:         calldata,
	}

	txn := extTxn.ToTransaction()

	payer := execution.NewDummyPayer()

	es.AddInTransaction(txn)
	res := es.HandleTransaction(ctx, txn, payer)
	if res.Failed() {
		return nil, fmt.Errorf("transaction failed: %w", res.GetError())
	}
	return res.ReturnData, nil
}

func (api *localShardApi) GetTokens1(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (map[types.TokenId]types.Value, error) {
	abi, err := contracts.GetAbi(contracts.NameTokenManager)
	if err != nil {
		return nil, fmt.Errorf("cannot get ABI: %w", err)
	}

	calldata, err := abi.Pack("getTokens", address)
	if err != nil {
		return nil, fmt.Errorf("cannot pack calldata: %w", err)
	}

	tokenManagerAddr := types.ShardAndHexToAddress(address.ShardId(), types.TokenManagerPureAddress)

	ret, err := api.CallGetter(ctx, tokenManagerAddr, calldata)
	if err != nil {
		return nil, fmt.Errorf("failed to call getter: %w", err)
	}

	var tokens []token
	err = abi.UnpackIntoInterface(&tokens, "getTokens", ret)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack response: %w", err)
	}

	res := make(map[types.TokenId]types.Value)
	for t := range tokens {
		res[types.TokenId(tokens[t].Token)] = types.NewValueFromBigMust(tokens[t].Balance)
	}
	return res, nil
}

func (api *LocalShardApi) CallGetter(
	ctx context.Context,
	address types.Address,
	calldata []byte,
) ([]byte, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	block, _, err := db.ReadLastBlock(tx, address.ShardId())
	if err != nil {
		return nil, fmt.Errorf("failed to read last block: %w", err)
	}

	cfgAccessor, err := config.NewConfigReader(tx, &block.MainShardHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create config accessor: %w", err)
	}

	es, err := execution.NewExecutionState(tx, address.ShardId(), execution.StateParams{
		Block:          block,
		ConfigAccessor: cfgAccessor,
		Mode:           execution.ModeReadOnly,
	})
	if err != nil {
		return nil, err
	}

	extTxn := &types.ExternalTransaction{
		FeeCredit:    types.GasToValue(types.DefaultMaxGasInBlock.Uint64()),
		MaxFeePerGas: types.MaxFeePerGasDefault,
		To:           address,
		Data:         calldata,
	}

	txn := extTxn.ToTransaction()

	payer := execution.NewDummyPayer()

	es.AddInTransaction(txn)
	res := es.HandleTransaction(ctx, txn, payer)
	if res.Failed() {
		return nil, fmt.Errorf("transaction failed: %w", res.GetError())
	}
	return res.ReturnData, nil
}

func (api *LocalShardApi) GetTokens1(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (map[types.TokenId]types.Value, error) {
	shardId := address.ShardId()
	if shardId != api.shardId() {
		return nil, api.errWrongShard()
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, _, err := api.getSmartContract(tx, address, blockReference, false)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, nil
	}

	tokenReader := execution.NewDbTokenTrieReader(tx, shardId)
	if err := tokenReader.SetRootHash(acc.TokenRoot); err != nil {
		return nil, err
	}
	entries, err := tokenReader.Entries()
	if err != nil {
		return nil, err
	}

	return common.SliceToMap(
		entries,
		func(_ int, kv execution.Entry[types.TokenId, *types.Value]) (types.TokenId, types.Value) {
			return kv.Key, *kv.Val
		}), nil
}

func (api *localShardApiRo) contractToRawEnriched(
	tx db.RoTx,
	contract *types.SmartContract,
	withCode bool,
	withStorage bool,
) (*rawapitypes.SmartContract, error) {
	shardId := contract.Address.ShardId()
	var err error
	var code types.Code
	if withCode {
		code, err = db.ReadCode(tx, shardId, contract.CodeHash)
		if err != nil {
			if !errors.Is(err, db.ErrKeyNotFound) {
				return nil, err
			}
			code = nil
		}
	}

	var storageEntries []execution.Entry[common.Hash, *types.Uint256]
	if withStorage {
		storageReader := execution.NewDbStorageTrieReader(tx, shardId)
		if err := storageReader.SetRootHash(contract.StorageRoot); err != nil {
			return nil, err
		}
		storageEntries, err = storageReader.Entries()
		if err != nil {
			return nil, err
		}
	}

	tokenReader := execution.NewDbTokenTrieReader(tx, shardId)
	if err := tokenReader.SetRootHash(contract.TokenRoot); err != nil {
		return nil, err
	}
	tokenEntries, err := tokenReader.Entries()
	if err != nil {
		return nil, err
	}

	asyncContextReader := execution.NewDbAsyncContextTrieReader(tx, shardId)
	if err := asyncContextReader.SetRootHash(contract.AsyncContextRoot); err != nil {
		return nil, err
	}
	asyncContextEntries, err := asyncContextReader.Entries()
	if err != nil {
		return nil, err
	}

	marshalledContract, err := contract.MarshalNil()
	if err != nil {
		return nil, err
	}

	return &rawapitypes.SmartContract{
		ContractBytes: marshalledContract,
		Code:          code,
		Storage:       execution.ConvertTrieEntriesToMap(storageEntries),
		Tokens:        execution.ConvertTrieEntriesToMap(tokenEntries),
		AsyncContext:  execution.ConvertTrieEntriesToMap(asyncContextEntries),
	}, nil
}

func (api *localShardApiRo) GetContract(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
	withCode bool,
	withStorage bool,
) (*rawapitypes.SmartContract, error) {
	shardId := address.ShardId()
	if shardId != api.shardId() {
		return nil, api.errWrongShard()
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	contract, proof, err := api.getSmartContract(tx, address, blockReference, true)
	if err != nil {
		return nil, err
	}

	encodedProof, err := proof.Encode()
	if err != nil {
		return nil, err
	}

	// If we don't have contract data, return just the proof
	if contract == nil {
		return &rawapitypes.SmartContract{ProofEncoded: encodedProof}, nil
	}

	contractRaw, err := api.contractToRawEnriched(tx, contract, withCode, withStorage)
	if err != nil {
		return nil, err
	}
	contractRaw.ProofEncoded = encodedProof
	return contractRaw, nil
}

// getSmartContract attempts to retrieve a smart contract from the database.
// If the contract exists, it returns the contract, `nil` otherwise.
// If `withProof` is true, it also returns a Merkle proof of existence or absence.
func (api *localShardApiRo) getSmartContract(
	tx db.RoTx,
	address types.Address,
	blockReference rawapitypes.BlockReference,
	withProof bool,
) (*types.SmartContract, *mpt.Proof, error) {
	rawBlock, err := api.getBlockByReference(tx, blockReference, false)
	if err != nil {
		return nil, nil, err
	}
	var block types.Block
	if err := block.UnmarshalNil(rawBlock.Block); err != nil {
		return nil, nil, err
	}

	reader := mpt.NewDbReader(tx, api.shardId(), db.ContractTrieTable)

	contractTrie := execution.NewContractTrieReader(reader)
	if err := contractTrie.SetRootHash(block.SmartContractsRoot); err != nil {
		return nil, nil, err
	}

	addressHash := address.Hash()
	var proof *mpt.Proof
	if withProof {
		// Create proof regardless of whether we have contract data
		proofValue, err := mpt.BuildProof(reader, addressHash.Bytes(), mpt.ReadOperation)
		if err != nil {
			return nil, nil, err
		}
		proof = &proofValue
	}

	contract, err := contractTrie.Fetch(addressHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			// there is no such contract, provide proof of absence
			return nil, proof, nil
		}
		return nil, nil, err
	}

	return contract, proof, nil
}

func (api *localShardApiRw) GetTransactionCount(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (uint64, error) {
	if blockReference.Type() == rawapitypes.NamedBlockIdentifierReference &&
		blockReference.NamedBlockIdentifier() == rawapitypes.PendingBlock {
		if api.txnpool != nil {
			seqno, inPool := api.txnpool.SeqnoToAddress(address)
			if inPool {
				seqno++
				return uint64(seqno), nil
			}
		}
		// Fallback to latest block if no transaction in pool
		blockReference = rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.LatestBlock)
	}

	tx, err := api.roApi.db.CreateRoTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, _, err := api.roApi.getSmartContract(tx, address, blockReference, false)
	if err != nil {
		return 0, err
	}
	if acc == nil {
		return 0, nil
	}
	return uint64(acc.ExtSeqno), nil
}

// GetRangeConfig defines a set of options that control which portions of the state
// are iterated and collected. As there is no formal standard for this functionality,
// we aim to replicate the behavior of go-ethereum (geth).
type GetRangeConfig struct {
	WithCode    bool
	WithStorage bool
	Start       common.Hash
	Max         uint64
}

// GetContractRange returnes a range off accounts starting from the next after `start` address.
func (api *localShardApiRo) GetContractRange(
	ctx context.Context,
	blockReference rawapitypes.BlockReference,
	start common.Hash,
	maxResults uint64,
	withCode bool,
	withStorage bool,
) (*rawapitypes.SmartContractRange, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	rawBlock, err := api.getBlockByReference(tx, blockReference, false)
	if err != nil {
		return nil, err
	}
	var block types.Block
	if err := block.UnmarshalNil(rawBlock.Block); err != nil {
		return nil, err
	}

	conf := &GetRangeConfig{
		WithCode:    withCode,
		WithStorage: withStorage,
		Start:       start,
		Max:         maxResults,
	}

	trieReader := execution.NewDbContractTrieReader(tx, api.shardId())
	if err := trieReader.SetRootHash(block.SmartContractsRoot); err != nil {
		return nil, err
	}

	contracts := make([]*rawapitypes.SmartContract, 0)
	var ctr uint64
	var nextKey *common.Hash
	for key, contract := range trieReader.ItemsFromKey(&conf.Start) {
		if ctr == conf.Max && conf.Max > 0 {
			nextKey = &key
			break
		}

		contractRaw, err := api.contractToRawEnriched(tx, &contract, conf.WithCode, conf.WithStorage)
		if err != nil {
			return nil, err
		}

		contracts = append(contracts, contractRaw)

		ctr++
	}

	return &rawapitypes.SmartContractRange{Contracts: contracts, Next: nextKey}, nil
}

func (api *localShardApiRo) GetStorageAt(
	ctx context.Context,
	address types.Address,
	key common.Hash,
	blockReference rawapitypes.BlockReference,
) (types.Uint256, error) {
	shardId := address.ShardId()
	if shardId != api.shardId() {
		return types.Uint256{}, api.errWrongShard()
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return types.Uint256{}, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, _, err := api.getSmartContract(tx, address, blockReference, false)
	if err != nil {
		return types.Uint256{}, err
	}
	if acc == nil {
		return types.Uint256{}, nil
	}

	contractWithStorage, err := api.contractToRawEnriched(tx, acc, false, true)
	if err != nil {
		return types.Uint256{}, err
	}

	return contractWithStorage.Storage[key], nil
}
