package faucet

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type API interface {
	TopUpViaFaucet(
		ctx context.Context, faucetAddress, contractAddressTo types.Address, amount types.Value) (common.Hash, error)
	CreateSmartAccount(
		ctx context.Context, shardId types.ShardId, pubKey hexutil.Bytes, salt types.Uint256, amount types.Value,
	) (types.Address, error)
	GetFaucets() map[string]types.Address
}

type APIImpl struct {
	client client.Client

	// Requests are served by one which is the easiest way to avoid seqno gaps.
	mu sync.Mutex
	// As long as we have only one faucet, we can manage seqnos locally
	// which can be more correct than getting tx count each time.
	seqnos map[types.Address]types.Seqno
}

var _ API = (*APIImpl)(nil)

func NewAPI(client client.Client) *APIImpl {
	return &APIImpl{
		client: client,
		seqnos: make(map[types.Address]types.Seqno),
	}
}

func (c *APIImpl) fetchSeqno(ctx context.Context, addr types.Address) (types.Seqno, error) {
	return c.client.GetTransactionCount(ctx, addr, transport.BlockNumberOrHash(transport.PendingBlock))
}

func (c *APIImpl) getOrFetchSeqno(ctx context.Context, faucetAddress types.Address) (types.Seqno, error) {
	// todo: currently, no better solution than to fetch seqno each time
	// Keeping in-memory seqno is not reliable because of possible desync with the chain.
	// seqno, ok := c.seqnos[faucetAddress]
	// if ok {
	//	return seqno, nil
	// }

	seqno, err := c.fetchSeqno(ctx, faucetAddress)
	if err != nil {
		return 0, err
	}

	c.seqnos[faucetAddress] = seqno

	return seqno, nil
}

func (c *APIImpl) TopUpViaFaucet(
	ctx context.Context,
	faucetAddress types.Address,
	contractAddressTo types.Address,
	amount types.Value,
) (common.Hash, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	seqno, err := c.getOrFetchSeqno(ctx, faucetAddress)
	if err != nil {
		return common.EmptyHash, err
	}

	contractName := contracts.NameFaucet
	if faucetAddress != types.FaucetAddress {
		contractName = contracts.NameFaucetToken
	}
	callData, err := contracts.NewCallData(contractName, "withdrawTo", contractAddressTo, amount.ToBig())
	if err != nil {
		return common.EmptyHash, err
	}
	extTxn := &types.ExternalTransaction{
		To:      faucetAddress,
		Data:    callData,
		Seqno:   seqno,
		Kind:    types.ExecutionTransactionKind,
		FeePack: types.NewFeePackFromGas(100_000),
	}

	data, err := extTxn.MarshalNil()
	if err != nil {
		return common.EmptyHash, err
	}

	hash, err := c.client.SendRawTransaction(ctx, data)
	if err != nil && !errors.Is(err, rpc.ErrRPCError) && !errors.Is(err, jsonrpc.ErrTransactionDiscarded) {
		return common.EmptyHash, err
	}
	if err != nil {
		actualSeqno, err2 := c.fetchSeqno(ctx, faucetAddress)
		if err2 != nil {
			return common.EmptyHash, fmt.Errorf(
				"failed to send transaction %d with %w and failed to get seqno: %w", seqno, err, err2)
		}

		extTxn.Seqno = actualSeqno
		data, err2 = extTxn.MarshalNil()
		if err2 != nil {
			return common.EmptyHash, err2
		}

		hash, err2 = c.client.SendRawTransaction(ctx, data)
		if err2 != nil {
			return common.EmptyHash, fmt.Errorf(
				"failed to send transaction %d with %w and then %d with %w", seqno, err, actualSeqno, err2)
		}

		seqno = actualSeqno
	}

	c.seqnos[faucetAddress] = seqno + 1

	return hash, nil
}

func (c *APIImpl) CreateSmartAccount(
	ctx context.Context, shardId types.ShardId, pubKey hexutil.Bytes, salt types.Uint256, amount types.Value,
) (types.Address, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	seqno, err := c.getOrFetchSeqno(ctx, types.FaucetAddress)
	if err != nil {
		return types.EmptyAddress, err
	}

	contractName := contracts.NameFaucet
	callData, err := contracts.NewCallData(contractName, "createSmartAccount",
		big.NewInt(int64(shardId)), []byte(pubKey), salt.Bytes32(), amount.ToBig())
	if err != nil {
		return types.EmptyAddress, err
	}
	extTxn := &types.ExternalTransaction{
		To:      types.FaucetAddress,
		Data:    callData,
		Seqno:   seqno,
		Kind:    types.ExecutionTransactionKind,
		FeePack: types.NewFeePackFromGas(5_000_000),
	}

	data, err := extTxn.MarshalNil()
	if err != nil {
		return types.EmptyAddress, err
	}

	hash, err := c.client.SendRawTransaction(ctx, data)
	if err != nil && !errors.Is(err, rpc.ErrRPCError) && !errors.Is(err, jsonrpc.ErrTransactionDiscarded) {
		return types.EmptyAddress, err
	}
	if err != nil {
		actualSeqno, err2 := c.fetchSeqno(ctx, types.FaucetAddress)
		if err2 != nil {
			return types.EmptyAddress, fmt.Errorf(
				"failed to send transaction %d with %w and failed to get seqno: %w", seqno, err, err2)
		}

		extTxn.Seqno = actualSeqno
		data, err2 = extTxn.MarshalNil()
		if err2 != nil {
			return types.EmptyAddress, err2
		}

		hash, err2 = c.client.SendRawTransaction(ctx, data)
		if err2 != nil {
			return types.EmptyAddress, fmt.Errorf(
				"failed to send transaction %d with %w and then %d with %w", seqno, err, actualSeqno, err2)
		}

		seqno = actualSeqno
	}

	c.seqnos[types.FaucetAddress] = seqno + 1

	var receipt *jsonrpc.RPCReceipt
	for {
		var err error
		receipt, err = c.client.GetInTransactionReceipt(ctx, hash)
		if err != nil {
			return types.EmptyAddress, fmt.Errorf("failed to get receipt for transaction %s: %w", hash, err)
		}
		if receipt.IsComplete() {
			break
		}
		select {
		case <-ctx.Done():
			return types.EmptyAddress, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	if receipt == nil || !receipt.AllSuccess() {
		return types.EmptyAddress, fmt.Errorf("transaction %s for contract deployment failed", hash)
	}

	smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(pubKey)
	code := types.BuildDeployPayload(smartAccountCode, salt.Bytes32())
	return types.CreateAddress(shardId, code), nil
}

func (c *APIImpl) GetFaucets() map[string]types.Address {
	return types.GetTokens()
}
