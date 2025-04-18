package internal

import (
	"context"
	"errors"
	"fmt"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"math/big"
)

var errBlockNotFound = errors.New("block not found")

func (api *localShardApiRo) GetBalance(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (types.Value, error) {
	shardId := address.ShardId()
	if shardId != api.shardId() {
		return types.Value{}, fmt.Errorf("address is not in the shard %d", api.shard)
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return types.Value{}, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, address, blockReference)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return types.Value{}, nil
		}
		return types.Value{}, err
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
		return types.Code{}, fmt.Errorf("address is not in the shard %d", api.shard)
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, address, blockReference)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
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

func (api *localShardApiRo) GetTokens(
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

	tokenManagerAddr := types.GetTokenManagerAddress(address.ShardId())

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

func (api *localShardApiRo) CallGetter(
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

func (api *localShardApiRo) GetContract(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (*rawapitypes.SmartContract, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	contractRaw, proofBuilder, err := api.getRawSmartContract(tx, address, blockReference)
	if err != nil && proofBuilder == nil {
		return nil, err
	}

	// Create proof regardless of whether we have contract data
	proof, err := proofBuilder(mpt.ReadMPTOperation)
	if err != nil {
		return nil, err
	}

	encodedProof, err := proof.Encode()
	if err != nil {
		return nil, err
	}

	// If we don't have contract data, return just the proof
	if contractRaw == nil {
		return &rawapitypes.SmartContract{ProofEncoded: encodedProof}, nil
	}

	contract := new(types.SmartContract)
	if err := contract.UnmarshalSSZ(contractRaw); err != nil {
		return nil, err
	}

	code, err := db.ReadCode(tx, address.ShardId(), contract.CodeHash)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
		code = nil
	}

	storageReader := execution.NewDbStorageTrieReader(tx, address.ShardId())
	storageReader.SetRootHash(contract.StorageRoot)
	storageEntries, err := storageReader.Entries()
	if err != nil {
		return nil, err
	}

	tokenReader := execution.NewDbTokenTrieReader(tx, address.ShardId())
	tokenReader.SetRootHash(contract.TokenRoot)
	tokenEntries, err := tokenReader.Entries()
	if err != nil {
		return nil, err
	}

	asyncContextReader := execution.NewDbAsyncContextTrieReader(tx, address.ShardId())
	asyncContextReader.SetRootHash(contract.AsyncContextRoot)
	asyncContextEntries, err := asyncContextReader.Entries()
	if err != nil {
		return nil, err
	}

	return &rawapitypes.SmartContract{
		ContractSSZ:  contractRaw,
		Code:         code,
		ProofEncoded: encodedProof,
		Storage:      execution.ConvertTrieEntriesToMap(storageEntries),
		Tokens:       execution.ConvertTrieEntriesToMap(tokenEntries),
		AsyncContext: execution.ConvertTrieEntriesToMap(asyncContextEntries),
	}, nil
}

type proofBuilder = func(operation mpt.MPTOperation) (mpt.Proof, error)

func makeProofBuilder(root *mpt.Reader, key []byte) proofBuilder {
	return func(operation mpt.MPTOperation) (mpt.Proof, error) {
		return mpt.BuildProof(root, key, operation)
	}
}

func (api *localShardApiRo) getRawSmartContract(
	tx db.RoTx,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) ([]byte, proofBuilder, error) {
	rawBlock, err := api.getBlockByReference(tx, blockReference, false)
	if err != nil {
		return nil, nil, err
	}
	if rawBlock == nil {
		return nil, nil, errBlockNotFound
	}
	var block types.Block
	if err := block.UnmarshalSSZ(rawBlock.Block); err != nil {
		return nil, nil, err
	}

	root := mpt.NewDbReader(tx, api.shardId(), db.ContractTrieTable)
	root.SetRootHash(block.SmartContractsRoot)
	addressBytes := address.Hash().Bytes()
	contractRaw, err := root.Get(addressBytes)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			// there is no such contract, provide proof of absence
			return nil, makeProofBuilder(root, addressBytes), err
		}
		return nil, nil, err
	}

	return contractRaw, makeProofBuilder(root, addressBytes), nil
}

func (api *localShardApiRo) getSmartContract(
	tx db.RoTx,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (*types.SmartContract, error) {
	contractRaw, _, err := api.getRawSmartContract(tx, address, blockReference)
	if err != nil {
		return nil, err
	}

	contract := new(types.SmartContract)
	if err := contract.UnmarshalSSZ(contractRaw); err != nil {
		return nil, err
	}

	return contract, nil
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

	acc, err := api.roApi.getSmartContract(tx, address, blockReference)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return uint64(acc.ExtSeqno), nil
}
