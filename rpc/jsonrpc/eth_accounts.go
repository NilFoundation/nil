package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
)

func (api *APIImpl) getSmartContract(tx db.Tx, shardId types.ShardId, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*types.SmartContract, error) {
	blockHash := common.EmptyHash
	switch {
	case blockNrOrHash.BlockNumber != nil && *blockNrOrHash.BlockNumber == transport.LatestBlockNumber:
		hashRaw, err := tx.Get(db.LastBlockTable, shardId.Bytes())
		if err != nil {
			return nil, err
		}

		blockHash = common.BytesToHash(*hashRaw)
	case blockNrOrHash.BlockNumber != nil:
		return nil, errNotImplemented
	case blockNrOrHash.BlockHash != nil:
		blockHash = *blockNrOrHash.BlockHash
	}

	block := db.ReadBlock(tx, shardId, blockHash)
	if block == nil {
		return nil, nil
	}

	root := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, db.ContractTrieTable, block.SmartContractsRoot)
	contractRaw, err := root.Get(address.Hash().Bytes())
	if contractRaw == nil || err != nil {
		return nil, err
	}

	contract := new(types.SmartContract)
	if err := contract.UnmarshalSSZ(contractRaw); err != nil {
		return nil, err
	}

	return contract, nil
}

// GetBalance implements eth_getBalance. Returns the balance of an account for a given address.
func (api *APIImpl) GetBalance(ctx context.Context, shardId types.ShardId, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Big, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, shardId, address, blockNrOrHash)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		// Special case - non-existent account is assumed to have zero balance
		return (*hexutil.Big)(big.NewInt(0)), nil
	}
	return (*hexutil.Big)(acc.Balance.ToBig()), nil
}

// GetTransactionCount implements eth_getTransactionCount. Returns the number of transactions sent from an address (the nonce / seqno).
func (api *APIImpl) GetTransactionCount(ctx context.Context, shardId types.ShardId, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Uint64, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	if blockNrOrHash.BlockNumber != nil && *blockNrOrHash.BlockNumber == transport.PendingBlockNumber {
		nonce, inPool := api.msgPools[shardId].SeqnoFromAddress(address)
		if inPool {
			nonce++
			return (*hexutil.Uint64)(&nonce), nil
		}
		// Fallback to latest block if no message in pool
		blockNrOrHash.BlockNumber = transport.LatestBlock.BlockNumber
	}
	zeroNonce := hexutil.Uint64(0)
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, shardId, address, blockNrOrHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return &zeroNonce, nil
		}

		return nil, err
	}
	if acc == nil {
		return &zeroNonce, nil
	}
	return (*hexutil.Uint64)(&acc.Seqno), nil
}

// GetCode implements eth_getCode. Returns the byte code at a given address (if it's a smart contract).
func (api *APIImpl) GetCode(ctx context.Context, shardId types.ShardId, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Bytes, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, shardId, address, blockNrOrHash)
	if acc == nil || err != nil {
		return nil, err
	}

	code, err := db.ReadCode(tx, shardId, acc.CodeHash)
	if code == nil || err != nil {
		return nil, err
	}
	return hexutil.Bytes(code), nil
}
